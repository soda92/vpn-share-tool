import socket
import sys
import json
import time
import logging
from urllib.parse import urlparse
from urllib.request import urlopen

from zeroconf import ServiceBrowser, Zeroconf, ServiceListener


class VpnShareListener(ServiceListener):
    """A listener for the VPN Share Tool mDNS service."""

    def __init__(self):
        self.service_info = None

    def remove_service(self, zeroconf, type, name):
        logging.debug(f"Service {name} removed")
        self.service_info = None

    def add_service(self, zeroconf, type, name):
        logging.debug(f"Service {name} added")
        info = zeroconf.get_service_info(type, name)
        if info:
            logging.debug(f"  Address: {socket.inet_ntoa(info.addresses[0])}")
            logging.debug(f"  Port: {info.port}")
            self.service_info = info

    def update_service(self, zeroconf, type, name):
        # For simplicity, we'll just treat update as a new addition
        self.add_service(zeroconf, type, name)


def discover_proxy(target_url, timeout=5):
    """
    Discovers a proxy for a given URL by browsing for the mDNS service.

    Args:
        target_url: The URL to find a proxy for.
        timeout: The time to wait for discovery in seconds.

    Returns:
        The proxy URL if found, otherwise None.
    """
    # 1. Discover the API server via mDNS
    zeroconf = Zeroconf()
    listener = VpnShareListener()
    browser = ServiceBrowser(zeroconf, "_vpnshare-api._tcp.local.", listener)

    logging.debug(f"Browsing for _vpnshare-api._tcp.local. for {timeout} seconds...")
    time.sleep(timeout) # Wait for discovery

    browser.cancel()
    zeroconf.close()

    if not listener.service_info:
        logging.info("Discovery timed out. No VPN Share API server found.")
        return None

    # 2. Query the API server to get the list of shared proxies
    info = listener.service_info
    server_ip = socket.inet_ntoa(info.addresses[0])
    server_port = info.port
    api_url = f"http://{server_ip}:{server_port}/services"

    try:
        logging.debug(f"Querying API server at {api_url}")
        with urlopen(api_url, timeout=5) as response:
            if response.status != 200:
                logging.error(f"API server returned status {response.status}")
                return None
            services = json.loads(response.read())
    except Exception as e:
        logging.error(f"Failed to get services from API server: {e}")
        return None

    # 3. Find the matching proxy from the list
    target_hostname = urlparse(target_url).hostname
    if not target_hostname:
        # Handle cases like 'localhost:8000' which might not have a scheme
        target_hostname = urlparse(f"http://{target_url}").hostname

    logging.debug(f"Looking for a proxy for hostname: {target_hostname}")
    for service in services:
        original_url_hostname = urlparse(service.get("original_url")).hostname
        if original_url_hostname == target_hostname:
            proxy_url = service.get("shared_url")
            logging.debug(f"Found matching proxy: {proxy_url}")
            return proxy_url

    logging.info(f"Found API server, but no proxy available for {target_url}")
    return None


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG, format='%(asctime)s - %(levelname)s - %(message)s')

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