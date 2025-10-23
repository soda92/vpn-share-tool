import logging
import socket
import sys
import json
import urllib.request
from threading import Event
from urllib.parse import urlparse

from zeroconf import ServiceBrowser, Zeroconf, ServiceListener


class VpnShareListener(ServiceListener):
    """A listener for the VPN Share Tool mDNS service."""

    def __init__(self, event):
        self.services = []
        self.found_event = event

    def remove_service(self, zc, type_, name):
        logging.debug(f"Service {name} removed")
        self.services = [s for s in self.services if s.name != name]

    def add_service(self, zc, type_, name):
        logging.debug(f"Service {name} added")
        info = zc.get_service_info(type_, name)
        if info:
            logging.debug(f"  Address: {socket.inet_ntoa(info.addresses[0])}")
            logging.debug(f"  Port: {info.port}")
            self.services.append(info)
            self.found_event.set()  # Signal that a service has been found

    def update_service(self, zc, type_, name):
        # For simplicity, we'll just treat update as a new addition
        self.add_service(zc, type_, name)


def discover_proxy(target_url, timeout=10):
    """
    Discovers a proxy for a given URL by browsing for the mDNS service,
    and requests its creation if it doesn't exist.

    Args:
        target_url: The URL to find a proxy for.
        timeout: The time to wait for discovery in seconds.

    Returns:
        The proxy URL if found, otherwise None.
    """
    # 1. Discover all available API servers via mDNS
    zeroconf = Zeroconf()
    found_event = Event()
    listener = VpnShareListener(found_event)
    browser = ServiceBrowser(zeroconf, "_vpnshare-api._tcp.local.", listener)

    logging.debug(f"Browsing for _vpnshare-api._tcp.local. for up to {timeout} seconds...")
    found_event.wait(timeout)
    logging.debug("Discovery window closed.")

    browser.cancel()
    zeroconf.close()

    if not listener.services:
        logging.info("Discovery timed out. No VPN Share API server found.")
        return None

    target_hostname = urlparse(target_url).hostname
    if not target_hostname:
        target_hostname = urlparse(f"http://{target_url}").hostname

    # 2. Phase 1: Check all discovered servers for an EXISTING proxy
    logging.debug("Phase 1: Checking for existing proxies...")
    for info in listener.services:
        server_ip = socket.inet_ntoa(info.addresses[0])
        server_port = info.port
        api_url = f"http://{server_ip}:{server_port}/services"

        try:
            logging.debug(f"Querying API server at {api_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(api_url, timeout=5) as response:
                if response.status != 200:
                    logging.warning(f"API server at {api_url} returned status {response.status}")
                    continue
                services = json.loads(response.read())

            # Find the matching proxy from the list for this server
            for service in services:
                original_url_hostname = urlparse(service.get("original_url")).hostname
                if original_url_hostname == target_hostname:
                    proxy_url = service.get("shared_url")
                    logging.debug(f"Found existing proxy: {proxy_url} on server {api_url}")
                    return proxy_url
        except Exception as e:
            logging.warning(f"Could not check services on {api_url}: {e}")
            continue

    # 3. Phase 2: No existing proxy found. Ask a capable server to create one.
    logging.debug("Phase 2: No existing proxy found. Requesting creation...")
    for info in listener.services:
        server_ip = socket.inet_ntoa(info.addresses[0])
        server_port = info.port

        # First, check if this server can reach the target URL
        try:
            can_reach_url = f"http://{server_ip}:{server_port}/can-reach?url={urllib.parse.quote(target_url)}"
            logging.debug(f"Checking reachability at {can_reach_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(can_reach_url, timeout=5) as response:
                if response.status != 200:
                    continue
                reach_data = json.loads(response.read())
                if not reach_data.get("reachable"):
                    logging.debug(f"Server {server_ip} cannot reach {target_url}")
                    continue
        except Exception as e:
            logging.warning(f"Could not check reachability on {server_ip}: {e}")
            continue

        # This server can reach the URL, so ask it to create the proxy
        logging.debug(f"Server {server_ip} can reach the URL. Requesting proxy creation...")
        try:
            create_url = f"http://{server_ip}:{server_port}/proxies"
            post_data = json.dumps({"url": target_url}).encode("utf-8")
            req = urllib.request.Request(create_url, data=post_data, headers={"Content-Type": "application/json"})
            
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(req, timeout=10) as response:
                if response.status == 201: # StatusCreated
                    new_proxy_data = json.loads(response.read())
                    proxy_url = new_proxy_data.get("shared_url")
                    logging.debug(f"Successfully created proxy: {proxy_url}")
                    return proxy_url
                else:
                    logging.error(f"Server {server_ip} failed to create proxy, status: {response.status}")
                    return None # If one server fails to create, stop trying
        except Exception as e:
            logging.error(f"Failed to request proxy creation from {server_ip}: {e}")
            return None # If one server fails to create, stop trying

    logging.info(f"Found API server(s), but none could reach or create a proxy for {target_url}")


if __name__ == "__main__":
    logging.basicConfig(
        level=logging.DEBUG, format="%(asctime)s - %(levelname)s - %(message)s"
    )

    if len(sys.argv) < 2:
        print(f"Usage: python3 {sys.argv[0]} <url_to_discover>", file=sys.stderr)
        sys.exit(1)

    url_to_discover = sys.argv[1]
    proxy_url_found = discover_proxy(url_to_discover)

    if proxy_url_found:
        # Print the proxy URL to stdout for scripting
        print(proxy_url_found, file=sys.stdout)
    else:
        # Exit with a non-zero status code if no proxy is found
        sys.exit(2)
