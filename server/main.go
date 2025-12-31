package main

func main() {
	loadTaggedURLs()
	// Start TCP server for vpn-share-tool instances
	go startTCPServer()
	// Start the automatic proxy creator
	go startAutoProxyCreator()
	// Start HTTP server for the web UI
	startHTTPServer()
}
