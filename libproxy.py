import logging
import socket
import sys
import json
import urllib.request
from urllib.parse import urlparse

# Address of the central discovery server
DISCOVERY_SERVER_HOST = "192.168.1.81"
DISCOVERY_SERVER_PORT = 45679


def get_instance_list():
    """Gets the list of active vpn-share-tool instances from the central server."""
    try:
        with socket.create_connection((DISCOVERY_SERVER_HOST, DISCOVERY_SERVER_PORT), timeout=5) as sock:
            sock.sendall(b"LIST\n")
            response = sock.makefile().readline()
            instances_raw = json.loads(response)
            # The server gives us a list of objects with an "address" field
            return [item["address"] for item in instances_raw]
    except Exception as e:
        logging.error(f"Failed to get instance list from discovery server: {e}")
        return []

def discover_proxy(target_url, timeout=10):
    """
    Discovers a proxy for a given URL by querying the central discovery server.
    """
    # 1. Get the list of all available API servers from the central server
    instance_addresses = get_instance_list()
    if not instance_addresses:
        logging.info("No active vpn-share-tool instances found.")
        return None

    target_hostname = urlparse(target_url).hostname
    if not target_hostname:
        target_hostname = urlparse(f"http://{target_url}").hostname

    # 2. Phase 1: Check all discovered servers for an EXISTING proxy
    logging.debug("Phase 1: Checking for existing proxies...")
    for instance_addr in instance_addresses:
        api_url = f"http://{instance_addr}/services"
        try:
            logging.debug(f"Querying API server at {api_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(api_url, timeout=5) as response:
                if response.status != 200:
                    logging.warning(f"API server at {api_url} returned status {response.status}")
                    continue
                services = json.loads(response.read())

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
    for instance_addr in instance_addresses:
        # First, check if this server can reach the target URL
        try:
            can_reach_url = f"http://{instance_addr}/can-reach?url={urllib.parse.quote(target_url)}"
            logging.debug(f"Checking reachability at {can_reach_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(can_reach_url, timeout=5) as response:
                if response.status != 200:
                    continue
                reach_data = json.loads(response.read())
                if not reach_data.get("reachable"):
                    logging.debug(f"Server {instance_addr} cannot reach {target_url}")
                    continue
        except Exception as e:
            logging.warning(f"Could not check reachability on {instance_addr}: {e}")
            continue

        # This server can reach the URL, so ask it to create the proxy
        logging.debug(f"Server {instance_addr} can reach the URL. Requesting proxy creation...")
        try:
            create_url = f"http://{instance_addr}/proxies"
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
                    logging.error(f"Server {instance_addr} failed to create proxy, status: {response.status}")
                    return None
        except Exception as e:
            logging.error(f"Failed to request proxy creation from {instance_addr}: {e}")
            return None

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
