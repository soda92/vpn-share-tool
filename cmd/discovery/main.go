package main

import (
	"flag"
	"log"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/posthog/posthog-go"
	"github.com/soda92/vpn-share-tool/discovery/api"
	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/store"
	"github.com/soda92/vpn-share-tool/discovery/transport"
)

func main() {
	insecure := flag.Bool("insecure", false, "Disable TLS and run on HTTP only")
	flag.Parse()

	// Initialize Sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              "https://bc888ace3f8f6751be2c1a8b8d71c71f@benefit.sodacris.com/4511405673480272",
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	defer sentry.Flush(2 * time.Second)

	// Recover panics with Sentry
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
			panic(r)
		}
	}()

	// Initialize PostHog
	phClient, err := posthog.NewWithConfig(
		"dummy",
		posthog.Config{
			Endpoint: "https://benefit.sodacris.com",
		},
	)
	if err != nil {
		log.Printf("Failed to initialize PostHog: %v", err)
	} else {
		defer phClient.Close()
		phClient.Enqueue(posthog.Capture{
			DistinctId: "discovery_server",
			Event:      "discovery_server_started",
		})
	}

	store.LoadTaggedURLs()
	// Start TCP server for vpn-share-tool instances
	go transport.StartTCPServer()
	// Start the automatic proxy creator
	go proxy.StartAutoProxyCreator()
	// Start HTTP server for the web UI
	api.StartHTTPServer(*insecure)
}
