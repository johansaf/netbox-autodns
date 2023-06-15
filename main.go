package main

import (
	"log"
	"net/http"
)

type Config struct {
	ListenAddress       string
	PdnsApiHost         string
	PdnsApiKey          string
	Domain              string
	CreateForwardRecord bool
	CreateReverseRecord bool
}

var cfg = readConfig()

func main() {
	var listenAddress = cfg.ListenAddress
	log.Printf("Listening on %s...\n", listenAddress)

	http.HandleFunc("/", handleWebhook)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
