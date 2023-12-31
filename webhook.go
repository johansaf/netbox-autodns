package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func updateRecord(webhook Webhook) error {
	// Delete the old reverse and forward record to prevent stale records
	// "invalid Prefix" means there's no old data available, we also don't want to delete the record if the dns name is empty
	if webhook.OldData.Prechange.Address.String() != "invalid Prefix" && webhook.OldData.Prechange.DnsName != "" {
		deleteRecord(webhook.OldData.Prechange.Address, webhook.OldData.Prechange.DnsName)
	}

	// Create new records
	zone, record, err := generateReverse(webhook.Data.Address)
	if err != nil {
		return fmt.Errorf("could not generate reverse zone and record: %s", err)
	}
	dnsName := ensureDot(webhook.Data.DnsName)

	// Create the new reverse record
	if !cfg.SkipReverseRecord {
		if err := cfg.PdnsClient.Records.Change(cfg.ctx, zone, record, powerdns.RRTypePTR, 86400, []string{dnsName}); err != nil {
			return fmt.Errorf("could not create new reverse record: %s", err)
		}
	}

	// Create the new forward record
	if !cfg.SkipForwardRecord {
		if strings.HasSuffix(dnsName, cfg.Domain) {
			if webhook.Data.Address.Addr().Is4() {
				if err := cfg.PdnsClient.Records.Change(cfg.ctx, cfg.Domain, dnsName, powerdns.RRTypeA, 86400, []string{webhook.Data.Address.Addr().String()}); err != nil {
					return fmt.Errorf("could not create new forward record: %s", err)
				}
			} else {
				if err := cfg.PdnsClient.Records.Change(cfg.ctx, cfg.Domain, dnsName, powerdns.RRTypeAAAA, 86400, []string{webhook.Data.Address.Addr().String()}); err != nil {
					return fmt.Errorf("could not create new forward record: %s", err)
				}
			}
		}
	}

	return nil
}

func deleteRecord(ip netip.Prefix, dnsName string) error {
	zone, record, err := generateReverse(ip)
	if err != nil {
		return fmt.Errorf("could not generate reverse zone and record: %s", err)
	}
	dnsName = ensureDot(dnsName)

	// Delete the old reverse record
	if !cfg.SkipReverseRecord {
		if err := cfg.PdnsClient.Records.Delete(cfg.ctx, zone, record, powerdns.RRTypePTR); err != nil {
			return fmt.Errorf("could not delete reverse record: %s", err)
		}
	}

	// Delete the old forward record
	if !cfg.SkipForwardRecord {
		if ip.Addr().Is4() {
			if err := cfg.PdnsClient.Records.Delete(cfg.ctx, cfg.Domain, dnsName, powerdns.RRTypeA); err != nil {
				return fmt.Errorf("could not delete forward record: %s", err)
			}
		} else {
			if err := cfg.PdnsClient.Records.Delete(cfg.ctx, cfg.Domain, dnsName, powerdns.RRTypeAAAA); err != nil {
				return fmt.Errorf("could not delete forward record: %s", err)
			}
		}
	}

	return nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	var webhook Webhook

	// We only accept POST requests
	// When deleting a record we should probably do DELETE, but I can't be bothered
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Verify the signature if it's present
	sig := r.Header.Get("X-Hook-Signature")
	if sig != "" {
		var buf bytes.Buffer
		// Quick and dirty way to read the body twice
		body := io.TeeReader(r.Body, &buf)
		if ok := verifySignature(sig, body); !ok {
			log.Println("Signature verification failed")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		err := json.NewDecoder(&buf).Decode(&webhook)
		if err != nil {
			log.Println("Could not decode request body to json")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	} else {
		// If there's no signature we can't verify the request, so we'll just assume it's valid
		err := json.NewDecoder(r.Body).Decode(&webhook)
		if err != nil {
			log.Println("Could not decode request body to json")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	// We should be good, set up the logger so we can follow the request a bit easier
	log.Printf("Received request %s\n", webhook.RequestId)
	log.SetPrefix(webhook.RequestId + " ")
	log.SetFlags(log.Lmsgprefix | log.LstdFlags)

	// Reset the log prefix and flags when we're done
	defer log.SetPrefix("")
	defer log.SetFlags(log.LstdFlags)

	cfg.PdnsClient = powerdns.NewClient(cfg.PdnsApiHost, "localhost", map[string]string{"X-API-Key": cfg.PdnsApiKey}, nil)
	cfg.ctx = context.Background()

	if webhook.Event == "created" || webhook.Event == "updated" {
		if err := updateRecord(webhook); err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if webhook.Event == "deleted" {
		deleteRecord(webhook.Data.Address, webhook.Data.DnsName)
	} else {
		// We should never get here
		log.Printf("Unknown event type: %s\n", webhook.Event)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We're done, return 200 OK
	w.WriteHeader(http.StatusOK)
}
