package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joeig/go-powerdns/v3"
)

type Config struct {
	ListenAddress     string
	PdnsApiHost       string
	PdnsApiKey        string
	Domain            string
	SkipForwardRecord bool
	SkipReverseRecord bool
	PdnsClient        *powerdns.Client
	ctx               context.Context
}

var cfg = readConfig()

func main() {
	var listenAddress = cfg.ListenAddress
	log.Printf("netbox-autodns commit %s starting\n", Commit[:7])
	log.Printf("Listening on %s...\n", listenAddress)

	http.HandleFunc("/", handleWebhook)
	http.HandleFunc("/hello", handleHello)

	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
