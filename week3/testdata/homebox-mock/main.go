// Run with: go run ./testdata/homebox-mock --config /private/tmp/homebox-demo.json
// The mock writes a private HomeBox MCP config and serves HTTPS until interrupted.
package main

import (
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
)

func main() {
	configFile := flag.String("config", "", "path to write a private MCP config")
	flag.Parse()
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "--config is required")
		os.Exit(2)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer demo-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/entities":
			if r.Method == "GET" {
				_, _ = w.Write([]byte(`{"items":[{"id":"item-1","name":"Desk lamp","quantity":1}],"page":1,"pageSize":20,"total":1}`))
				return
			}
			if r.Method == "POST" {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"item-2","name":"Created item"}`))
				return
			}
		case "/api/v1/entities/item-1":
			if r.Method == "GET" || r.Method == "PATCH" {
				_, _ = w.Write([]byte(`{"id":"item-1","name":"Desk lamp","quantity":1}`))
				return
			}
		case "/api/v1/entity-types":
			_, _ = w.Write([]byte(`[{"id":"type-1","name":"Item"}]`))
			return
		case "/api/v1/tags":
			_, _ = w.Write([]byte(`[{"id":"tag-1","name":"Lighting"}]`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()
	caFile := *configFile + ".ca.pem"
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		panic(err)
	}
	data, _ := json.MarshalIndent(map[string]string{"url": server.URL, "apiKey": "demo-key", "caFile": caFile}, "", "  ")
	if err := os.MkdirAll(filepath.Dir(*configFile), 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*configFile, data, 0600); err != nil {
		panic(err)
	}
	fmt.Printf("Mock HomeBox: %s\nPrivate config: %s\n", server.URL, *configFile)
	select {}
}
