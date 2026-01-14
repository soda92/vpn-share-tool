import logging
import socket
import sys
import json
import urllib.request
from urllib.parse import urlparse
import urllib.parse
import urllib.error
import ipaddress
import concurrent.futures
import subprocess
import re
import platform
import ssl
import os
from pathlib import Path

# Fallback addresses if scanning fails
DISCOVERY_SERVER_HOSTS = ["192.168.0.81", "192.168.1.81"]
DISCOVERY_SERVER_PORT = 45679

# Placeholder for CA Cert injection
CA_CERT_PEM = """__CA_CERT_PLACEHOLDER__"""


def get_cache_path():
    """Returns the path to the cache file."""
    if platform.system() == "Windows":
        base_dir = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
    else:
        base_dir = Path.home() / ".config"

    return base_dir / "vpn-share-tool" / "libproxy_cache.json"


def load_cache():
    """Loads the proxy cache from disk."""
    path = get_cache_path()
    if not path.exists():
        return {}
    try:
        with open(path, "r") as f:
            return json.load(f)
    except Exception as e:
        logging.debug(f"Failed to load cache: {e}")
        return {}


def save_to_cache(target_url, proxy_url):
    """Saves a discovered proxy to the cache."""
    path = get_cache_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        cache = load_cache()
        cache[target_url] = proxy_url
        with open(path, "w") as f:
            json.dump(cache, f)
    except Exception as e:
        logging.debug(f"Failed to save cache: {e}")


def get_local_ip():
    """Attempts to detect the local IP address, preferring private networks (192.168.x.x)."""

    # 1. Try parsing OS commands to find a 192.168.x.x address
    try:
        system = platform.system()
        if system == "Linux":
            output = subprocess.check_output(["ip", "addr"], text=True)
            # Look for 'inet 192.168.X.X/XX'
            matches = re.findall(r"inet\s+(192\.168\.\d+\.\d+)", output)
            if matches:
                return matches[0]
        elif system == "Windows":
            output = subprocess.check_output(["ipconfig"], text=True)
            # Look for 'IPv4 Address. . . . . . . . . . . : 192.168.X.X'
            matches = re.findall(r"IPv4 Address[ .]+:\s+(192\.168\.\d+\.\d+)", output)
            if matches:
                return matches[0]
    except (subprocess.CalledProcessError, FileNotFoundError) as e:
        logging.debug(f"Failed to parse OS network config: {e}")

    # 2. Fallback to default route (e.g. 8.8.8.8)
    try:
        # We don't actually send data, just opening the socket is enough to get the local IP
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
            s.connect(("8.8.8.8", 80))
            return s.getsockname()[0]
    except socket.error:
        return None


def scan_subnet(local_ip, port):
    """Scans the /24 subnet of the given IP for the specified TCP port."""
    if not local_ip:
        return []

    try:
        # Calculate the network
        ip_net = ipaddress.IPv4Network(f"{local_ip}/24", strict=False)
    except ValueError:
        return []

    # Skip 10.x.x.x networks as requested
    if str(ip_net.network_address).startswith("10."):
        logging.debug(f"Skipping scan for 10.x.x.x network: {ip_net}")
        return []

    found_hosts = []

    def check_host(ip):
        ip_str = str(ip)
        # Skip self if needed, but sometimes helpful
        try:
            with socket.create_connection((ip_str, port), timeout=0.2):
                return ip_str
        except (socket.timeout, ConnectionRefusedError, OSError):
            return None

    # Use a thread pool to scan quickly
    with concurrent.futures.ThreadPoolExecutor(max_workers=50) as executor:
        futures = [executor.submit(check_host, ip) for ip in ip_net.hosts()]
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            if result:
                found_hosts.append(result)

    return found_hosts


def get_instance_list(timeout: int = 5):
    """Gets the list of active vpn-share-tool instances using scanning and fallbacks."""

    # 1. Scan local subnet first
    local_ip = get_local_ip()
    candidate_hosts = []

    if local_ip:
        logging.debug(f"Detected local IP: {local_ip}. Scanning subnet...")
        scanned_hosts = scan_subnet(local_ip, DISCOVERY_SERVER_PORT)
        if scanned_hosts:
            logging.info(f"Found discovery servers via scanning: {scanned_hosts}")
            candidate_hosts.extend(scanned_hosts)
        else:
            logging.debug("No servers found via scanning.")
    else:
        logging.warning("Could not detect local IP for scanning.")

    # 2. Add fallbacks
    candidate_hosts.extend(DISCOVERY_SERVER_HOSTS)

    # Setup SSL context
    global CA_CERT_PEM
    if CA_CERT_PEM:
        if "__CA_CERT_PLACEHOLDER__" in CA_CERT_PEM:
            CA_FILE = Path(os.environ.get("VPN_SHARE_TOOL_CA_PATH", Path(__file__).resolve().parent.parent.joinpath("certs/ca.crt")))
            if CA_FILE.exists():
                CA_CERT_PEM = CA_FILE.read_text(encoding='utf8')
            else:
                logging.error("No CA certificate found. Exiting for security.")
                sys.exit(1)
        try:
            context = ssl.create_default_context(cadata=CA_CERT_PEM)
            context.check_hostname = False # Discovery uses IP/different hostname often
            logging.debug("Using embedded CA certificate for TLS.")
        except Exception as e:
            logging.error(
                f"Failed to load embedded CA cert: {e}. Exiting for security."
            )
            sys.exit(1)


    # 3. Try to connect to candidates
    for host in candidate_hosts:
        try:
            logging.debug(f"Trying discovery server at {host}...")
            with socket.create_connection(
                (host, DISCOVERY_SERVER_PORT), timeout=timeout
            ) as sock:
                with context.wrap_socket(sock, server_hostname=host) as ssock:
                    ssock.sendall(b"LIST\n")
                    response = ssock.makefile().readline()
                    if not response:
                        logging.warning(
                            f"Did not receive a response from discovery server at {host}"
                        )
                        continue
                    instances_raw = json.loads(response)
                    # The server gives us a list of objects with an "address" field
                    logging.info(f"Successfully retrieved instances from {host}")
                    return [item["address"] for item in instances_raw]
        except ssl.SSLError as e:
            logging.debug(f"SSL Error connecting to {host}: {e}")
            continue
        except socket.timeout:
            logging.debug(
                f"Timeout connecting to discovery server at {host}:{DISCOVERY_SERVER_PORT}"
            )
            continue
        except ConnectionRefusedError:
            logging.debug(
                f"Connection refused by discovery server at {host}:{DISCOVERY_SERVER_PORT}"
            )
            continue
        except json.JSONDecodeError as e:
            logging.error(
                f"Failed to decode JSON response from discovery server at {host}: {e}"
            )
            continue
        except Exception as e:
            logging.error(
                f"An unexpected error occurred while getting instance list from {host}: {e}"
            )
            continue

    logging.error("Failed to connect to any discovery server.")
    return []


def is_url_reachable_locally(target_url, timeout=3):
    """Checks if a URL is reachable from the local machine."""
    try:
        # Use a HEAD request for efficiency
        req = urllib.request.Request(target_url, method="HEAD")
        # Use a proxy handler that does nothing, to ensure we are checking direct access
        proxy_handler = urllib.request.ProxyHandler({})
        opener = urllib.request.build_opener(proxy_handler)
        with opener.open(req, timeout=timeout) as response:
            # Any status code means it's reachable.
            logging.debug(
                f"Local check: URL {target_url} is reachable with status {response.status}"
            )
            return True
    except (urllib.error.URLError, socket.timeout) as e:
        logging.debug(f"Local check: URL {target_url} is not reachable: {e}")
        return False


def discover_proxy(target_url, timeout=10, remote_only: bool=False):
    """
    Discovers a proxy for a given URL by querying the central discovery server.
    First, it checks if the URL is reachable locally.
    """
    # 0. Check for local reachability first
    logging.debug(f"Checking if {target_url} is reachable locally...")

    # Ensure the URL has a scheme for the request library
    schemed_target_url = target_url
    if not urlparse(schemed_target_url).scheme:
        schemed_target_url = f"http://{schemed_target_url}"

    if not remote_only:
        if is_url_reachable_locally(schemed_target_url, timeout=timeout):
            logging.info(f"URL {target_url} is directly reachable. No proxy needed.")
            return target_url  # Return the original URL

    # 0.5 Check cache
    cache = load_cache()
    if schemed_target_url in cache:
        cached_proxy = cache[schemed_target_url]
        logging.debug(f"Found cached proxy: {cached_proxy}")
        # Verify the cached proxy is still reachable
        if is_url_reachable_locally(cached_proxy, timeout=2):
            logging.info(f"Using cached proxy: {cached_proxy}")
            return cached_proxy
        else:
            logging.debug("Cached proxy not reachable, discarding.")
            # We could remove it from cache here, but overwrite later handles it

    logging.debug(
        f"URL {target_url} not reachable locally and no valid cache. Starting discovery..."
    )

    # 1. Get the list of all available API servers from the central server
    instance_addresses = get_instance_list(timeout=timeout)
    if not instance_addresses:
        logging.info("No active vpn-share-tool instances found.")
        return None

    target_hostname = urlparse(schemed_target_url).hostname

    # 2. Phase 1: Check all discovered servers for an EXISTING proxy
    logging.debug("Phase 1: Checking for existing proxies...")
    for instance_addr in instance_addresses:
        api_url = f"http://{instance_addr}/services"
        try:
            logging.debug(f"Querying API server at {api_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(api_url, timeout=timeout) as response:
                if response.status != 200:
                    logging.warning(
                        f"API server at {api_url} returned status {response.status}"
                    )
                    continue
                services = json.loads(response.read())

            if services:  # Ensure services is not None
                for service in services:
                    original_url_hostname = urlparse(
                        service.get("original_url")
                    ).hostname
                    if original_url_hostname == target_hostname:
                        proxy_url = service.get("shared_url")
                        logging.debug(
                            f"Found existing proxy: {proxy_url} on server {api_url}"
                        )
                        save_to_cache(schemed_target_url, proxy_url)
                        return proxy_url
        except Exception as e:
            logging.warning(f"Could not check services on {api_url}: {e}")
            continue

    # 3. Phase 2: No existing proxy found. Ask a capable server to create one.
    logging.debug("Phase 2: No existing proxy found. Requesting creation...")
    for instance_addr in instance_addresses:
        # First, check if this server can reach the target URL
        try:
            can_reach_url = f"http://{instance_addr}/can-reach?url={urllib.parse.quote(schemed_target_url)}"
            logging.debug(f"Checking reachability at {can_reach_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(can_reach_url, timeout=timeout) as response:
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
        logging.debug(
            f"Server {instance_addr} can reach the URL. Requesting proxy creation..."
        )
        try:
            create_url = f"http://{instance_addr}/proxies"
            post_data = json.dumps({"url": schemed_target_url}).encode("utf-8")
            req = urllib.request.Request(
                create_url,
                data=post_data,
                headers={"Content-Type": "application/json"},
            )

            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(req, timeout=timeout) as response:
                if response.status == 201:  # StatusCreated
                    new_proxy_data = json.loads(response.read())
                    proxy_url = new_proxy_data.get("shared_url")
                    logging.debug(f"Successfully created proxy: {proxy_url}")
                    save_to_cache(schemed_target_url, proxy_url)
                    return proxy_url
                else:
                    logging.error(
                        f"Server {instance_addr} failed to create proxy, status: {response.status}"
                    )
                    continue  # Try next instance
        except Exception as e:
            logging.error(f"Failed to request proxy creation from {instance_addr}: {e}")
            continue  # Try next instance

    logging.fatal(
        f"Found API server(s), but none could reach or create a proxy for {target_url}"
    )
    sys.exit(-1)


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
