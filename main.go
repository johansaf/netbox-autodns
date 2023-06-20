package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joeig/go-powerdns/v3"
)

// Struct containing configuration values, globally available
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

var cfg = Config{}

func main() {
	cfg = readConfig()

	var listenAddress = cfg.ListenAddress
	log.Printf("Listening on %s...\n", listenAddress)

	http.HandleFunc("/", handleWebhook)
	http.HandleFunc("/hello", handleHello)

	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
