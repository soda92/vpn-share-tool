package main

import "github.com/soda92/vpn-share-tool/discovery"

func main() {
	discovery.LoadTaggedURLs()
	// Start TCP server for vpn-share-tool instances
	go discovery.StartTCPServer()
	// Start the automatic proxy creator
	go discovery.StartAutoProxyCreator()
	// Start HTTP server for the web UI
	discovery.StartHTTPServer()
}
