package main

import (
	"github.com/soda92/vpn-share-tool/discovery/api"
	"github.com/soda92/vpn-share-tool/discovery/proxy"
	"github.com/soda92/vpn-share-tool/discovery/store"
	"github.com/soda92/vpn-share-tool/discovery/transport"
)

func main() {
	store.LoadTaggedURLs()
	// Start TCP server for vpn-share-tool instances
	go transport.StartTCPServer()
	// Start the automatic proxy creator
	go proxy.StartAutoProxyCreator()
	// Start HTTP server for the web UI
	api.StartHTTPServer()
}
