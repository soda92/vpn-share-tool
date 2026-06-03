package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/posthog/posthog-go"
	"github.com/soda92/vpn-share-tool/common"
	"github.com/soda92/vpn-share-tool/discovery/api"
	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/store"
	"github.com/soda92/vpn-share-tool/discovery/transport"
)

func main() {
	insecure := flag.Bool("insecure", false, "Disable TLS and run on HTTP only")
	resetPass := flag.String("reset-password", "", "Reset password for the specified user (e.g. -reset-password mypass)")
	resetUser := flag.String("reset-user", "admin", "User to reset password for")
	flag.Parse()

	if *resetPass != "" {
		store.LoadUsers()
		err := store.UpdatePassword(*resetUser, *resetPass)
		if err != nil {
			log.Fatalf("Failed to reset password for user %s: %v", *resetUser, err)
		}
		log.Printf("Successfully reset password for user %s.", *resetUser)
		os.Exit(0)
	}


	// Initialize Sentry
	sentryDsn := common.SentryDSN
	if sentryDsn == "" {
		sentryDsn = os.Getenv("VITE_SENTRY_DSN")
	}
	if sentryDsn != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDsn,
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
	} else {
		log.Println("VITE_SENTRY_DSN environment variable not set, Sentry integration disabled.")
	}

	// Initialize PostHog
	phKey := common.PosthogKey
	if phKey == "" {
		phKey = os.Getenv("VITE_POSTHOG_KEY")
	}
	if phKey != "" {
		phEndpoint := common.PosthogHost
		if phEndpoint == "" {
			phEndpoint = os.Getenv("VITE_POSTHOG_HOST")
		}
		if phEndpoint == "" {
			phEndpoint = "https://us.i.posthog.com"
		}
		phClient, err := posthog.NewWithConfig(
			phKey,
			posthog.Config{
				Endpoint: phEndpoint,
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
	} else {
		log.Println("VITE_POSTHOG_KEY environment variable not set, PostHog integration disabled.")
	}

	store.LoadTaggedURLs()
	store.LoadUsers()
	store.InitDefaultUser()
	// Start TCP server for vpn-share-tool instances
	go transport.StartTCPServer()
	// Start the automatic proxy creator
	go proxy.StartAutoProxyCreator()
	// Start HTTP server for the web UI
	api.StartHTTPServer(*insecure)
}
