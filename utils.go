package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// Used for health check
func handleHello(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Hello string
	}{
		Hello: "World!",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// https://golangcookbook.com/chapters/strings/reverse/
func reverse(s string) string {
	chars := []rune(s)
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}

// Returns the reverse zone and record for a given prefix
func generateReverse(prefix netip.Prefix) (string, string, error) {
	var zone string
	var record string

	if prefix.Addr().Is4() {
		tmp := strings.Split(prefix.Addr().String(), ".")
		zone = tmp[2] + "." + tmp[1] + "." + tmp[0] + ".in-addr.arpa." // Assume a /24 prefix
		record = tmp[3] + "." + tmp[2] + "." + tmp[1] + "." + tmp[0] + ".in-addr.arpa."
	} else if prefix.Addr().Is6() {
		// Change this to modify the subnet size
		tmp := prefix.Addr().StringExpanded()[0:9]
		zone = reverse(strings.Join(strings.Split(strings.ReplaceAll(tmp, ":", ""), ""), ".")) + ".ip6.arpa."
		record = reverse(strings.Join(strings.Split(strings.ReplaceAll(prefix.Addr().StringExpanded(), ":", ""), ""), ".")) + ".ip6.arpa."
	} else {
		return "", "", fmt.Errorf("prefix is neither IPv4 nor IPv6")
	}

	return zone, record, nil
}

// Verify the HMAC signature of the request
func verifySignature(sig string, reqBody io.Reader) bool {
	body, err := io.ReadAll(reqBody)
	if err != nil {
		log.Println("Could not read request body in verifySignature")
		return false
	}
	h := hmac.New(sha512.New, []byte(cfg.Secret))
	h.Write([]byte(body))
	sha := hex.EncodeToString((h.Sum(nil)))
	return sha == sig
}

// Make sure the DNS name ends with a dot, which is required by PowerDNS
func ensureDot(s string) string {
	if s[len(s)-1:] != "." {
		return s + "."
	}
	return s
}

// Function to read environment variables and put them into a Config struct
func readConfig() Config {
	// Check if the LISTEN_ADDRESS environment variable is empty, set it to :8080 if so
	if os.Getenv("LISTEN_ADDRESS") == "" {
		os.Setenv("LISTEN_ADDRESS", ":8080")
	}

	// Check if the PDNS_API_HOST, PDNS_API_KEY or DOMAIN environment variables are empty, if they are we log an error
	if os.Getenv("PDNS_API_HOST") == "" || os.Getenv("PDNS_API_KEY") == "" || os.Getenv("DOMAIN") == "" {
		log.Fatal("PDNS_API_HOST, PDNS_API_KEY or DOMAIN environment variables are empty")
	}

	// Don't create reverse records if SKIP_REVERSE_RECORDS is not empty
	if os.Getenv("SKIP_REVERSE_RECORDS") != "" {
		os.Setenv("SKIP_REVERSE_RECORDS", "true")
	}

	// Don't create forward records if SKIP_FORWARD_RECORDS is not empty
	if os.Getenv("SKIP_FORWARD_RECORDS") != "" {
		os.Setenv("SKIP_FORWARD_RECORDS", "true")
	}

	cfg := Config{
		ListenAddress:     os.Getenv("LISTEN_ADDRESS"),
		PdnsApiHost:       os.Getenv("PDNS_API_HOST"),
		PdnsApiKey:        os.Getenv("PDNS_API_KEY"),
		Domain:            ensureDot(os.Getenv("DOMAIN")),
		Secret:            os.Getenv("SECRET"),
		SkipForwardRecord: os.Getenv("SKIP_FORWARD_RECORDS") == "true",
		SkipReverseRecord: os.Getenv("SKIP_REVERSE_RECORDS") == "true",
	}
	return cfg
}
