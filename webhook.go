package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"strings"

	"github.com/joeig/go-powerdns/v3"
)

type Webhook struct {
	Event     string `json:"event"`
	Model     string `json:"model"`
	RequestId string `json:"request_id"`
	Data      struct {
		Address netip.Prefix `json:"address"`
		DnsName string       `json:"dns_name"`
	} `json:"data"`
	OldData struct {
		Prechange struct {
			Address netip.Prefix `json:"address"`
			DnsName string       `json:"dns_name"`
		} `json:"prechange"`
	} `json:"snapshots"`
}

func updateRecord(client *powerdns.Client, ctx context.Context, webhook Webhook) error {
	// Delete the old reverse and forward record to prevent stale records
	// "invalid Prefix" means there's no old data available, we also don't want to delete the record if the dns name is empty
	if webhook.OldData.Prechange.Address.String() != "invalid Prefix" && webhook.OldData.Prechange.DnsName != "" {
		deleteRecord(client, ctx, webhook.OldData.Prechange.Address, webhook.OldData.Prechange.DnsName)
	}

	// Create new records
	zone, record, err := generateReverse(webhook.Data.Address)
	if err != nil {
		return fmt.Errorf("could not generate reverse zone and record: %s", err)
	}
	dnsName := ensureDot(webhook.Data.DnsName)

	// Create the new reverse record
	if cfg.SkipReverseRecord {
		if err := client.Records.Change(ctx, zone, record, powerdns.RRTypePTR, 86400, []string{dnsName}); err != nil {
			return fmt.Errorf("could not create new reverse record: %s", err)
		}
	}

	// Create the new forward record
	if cfg.SkipForwardRecord {
		domain := "." + ensureDot(cfg.Domain) // Prefix the DOMAIN with a dot to prevent issues with missing dots, like "router.example.com" becoming "routerexample.com"
		if strings.HasSuffix(dnsName, domain) {
			if webhook.Data.Address.Addr().Is4() {
				if err := client.Records.Change(ctx, ensureDot(cfg.Domain), dnsName, powerdns.RRTypeA, 86400, []string{webhook.Data.Address.Addr().String()}); err != nil {
					return fmt.Errorf("could not create new forward record: %s", err)
				}
			} else {
				if err := client.Records.Change(ctx, ensureDot(cfg.Domain), dnsName, powerdns.RRTypeAAAA, 86400, []string{webhook.Data.Address.Addr().String()}); err != nil {
					return fmt.Errorf("could not create new forward record: %s", err)
				}
			}
		}
	}

	return nil
}

func deleteRecord(client *powerdns.Client, ctx context.Context, ip netip.Prefix, dnsName string) error {
	zone, record, err := generateReverse(ip)
	if err != nil {
		return fmt.Errorf("could not generate reverse zone and record: %s", err)
	}
	dnsName = ensureDot(dnsName)

	// Delete the old reverse record
	if cfg.SkipReverseRecord {
		if err := client.Records.Delete(ctx, zone, record, powerdns.RRTypePTR); err != nil {
			return fmt.Errorf("could not delete reverse record: %s", err)
		}
	}

	// Delete the old forward record
	if cfg.SkipForwardRecord {
		if ip.Addr().Is4() {
			if err := client.Records.Delete(ctx, ensureDot(cfg.Domain), dnsName, powerdns.RRTypeA); err != nil {
				return fmt.Errorf("could not delete forward record: %s", err)
			}
		} else {
			if err := client.Records.Delete(ctx, ensureDot(cfg.Domain), dnsName, powerdns.RRTypeAAAA); err != nil {
				return fmt.Errorf("could not delete forward record: %s", err)
			}
		}
	}

	return nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	// We only accept POST requests
	// When deleting a record we should probably do DELETE, but I can't be bothered
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Decode the JSON data into a struct that we can work with
	var webhook Webhook
	err := json.NewDecoder(r.Body).Decode(&webhook)
	if err != nil {
		log.Println("Could not decode request body")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We should be good, set up the logger so we can follow the request a bit easier
	log.Printf("Received request %s\n", webhook.RequestId)
	log.SetPrefix(webhook.RequestId + " ")
	log.SetFlags(log.Lmsgprefix | log.LstdFlags)

	// Reset the log prefix and flags when we're done
	defer log.SetPrefix("")
	defer log.SetFlags(log.LstdFlags)

	pdns := powerdns.NewClient(cfg.PdnsApiHost, "localhost", map[string]string{"X-API-Key": cfg.PdnsApiKey}, nil)
	ctx := context.Background()

	if webhook.Event == "created" || webhook.Event == "updated" {
		if err := updateRecord(pdns, ctx, webhook); err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if webhook.Event == "deleted" {
		deleteRecord(pdns, ctx, webhook.Data.Address, webhook.Data.DnsName)
	} else {
		// We should never get here
		log.Printf("Unknown event type: %s\n", webhook.Event)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We're done, return 200 OK
	w.WriteHeader(http.StatusOK)
}
