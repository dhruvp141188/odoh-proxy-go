// The MIT License
//
// Copyright (c) 2019 Apple, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cisco/go-hpke"
)

const (
	// HPKE constants
	kemID  = hpke.DHKEM_X25519
	kdfID  = hpke.KDF_HKDF_SHA256
	aeadID = hpke.AEAD_AESGCM128

	// HTTP constants. Fill in your proxy and target here.
	defaultPort    = "8080"
	proxyURI       = "http://localhost"
	queryEndpoint  = "/proxy"
	healthEndpoint = "/health"
	configEndpoint = "/.well-known/odohconfigs"

	// WebPvD configuration. Fill in your values here.
	webPvDString = `"{ "identifier" : "github.com", "expires" : "2019-08-23T06:00:00Z", "prefixes" : [ ], "dnsZones" : [ "odoh.example.net" ] }"`

	// Environment variables
	secretSeedEnvironmentVariable    = "SEED_SECRET_KEY"
	targetNameEnvironmentVariable    = "TARGET_INSTANCE_NAME"
	experimentIDEnvironmentVariable  = "EXPERIMENT_ID"
	telemetryTypeEnvironmentVariable = "TELEMETRY_TYPE"
)

var (
	// DNS constants. Fill in a DNS server to forward to here.
	nameServers = []string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"}
)

type odohServer struct {
	endpoints map[string]string
	Verbose   bool
	proxy     *proxyServer
	DOHURI    string
}

func (s odohServer) queryHandler(w http.ResponseWriter, r *http.Request) {
	targetName := r.URL.Query().Get("targethost")
	targetPath := r.URL.Query().Get("targetpath")
	if targetName != "" {
		if targetPath == "" {
			targetPath = queryEndpoint
		}
		s.proxy.proxyQueryHandler(w, r)
	} else {
		log.Printf("targethost empty, but targetpath specified: this is invalid")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}
}

func (s odohServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s Handling %s\n", r.Method, r.URL.Path)
	fmt.Fprint(w, "ODOH service\n")
	fmt.Fprint(w, "----------------\n")
	fmt.Fprintf(w, "Proxy endpoint: https://%s:%s/%s{?targethost,targetpath}\n", r.URL.Hostname(), r.URL.Port(), s.endpoints[queryEndpoint])
	fmt.Fprintf(w, "Target endpoint: https://%s:%s/%s{?dns}\n", r.URL.Hostname(), r.URL.Port(), s.endpoints[queryEndpoint])
	fmt.Fprint(w, "----------------\n")
}

func (s odohServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s Handling %s\n", r.Method, r.URL.Path)
	fmt.Fprint(w, "ok")
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	var seed []byte
	if seedHex := os.Getenv(secretSeedEnvironmentVariable); seedHex != "" {
		log.Printf("Using Secret Key Seed : [%v]", seedHex)
		var err error
		seed, err = hex.DecodeString(seedHex)
		if err != nil {
			panic(err)
		}
	} else {
		seed = make([]byte, 16)
		rand.Read(seed)
	}

	var serverName string
	if serverNameSetting := os.Getenv(targetNameEnvironmentVariable); serverNameSetting != "" {
		serverName = serverNameSetting
	} else {
		serverName = "localhost"
	}
	log.Printf("Setting Server Name as %v", serverName)

	endpoints := make(map[string]string)
	endpoints["Proxy"] = queryEndpoint

	proxy := &proxyServer{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 1024,
				TLSHandshakeTimeout: 0 * time.Second,
			},
		},
	}

	server := odohServer{
		endpoints: endpoints,
		proxy:     proxy,
		DOHURI:    fmt.Sprintf("%s/%s", proxyURI, queryEndpoint),
	}

    http.HandleFunc(queryEndpoint, server.queryHandler)

	log.Printf("Listening on port %v\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
