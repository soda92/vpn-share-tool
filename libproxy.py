import socket
import sys
from urllib.parse import urlparse
import logging


def discover_proxy(target_url, discovery_port=45678, timeout=10):
    """
    Discovers a proxy for a given URL by sending a UDP broadcast.

    Args:
        target_url: The URL to find a proxy for.
        discovery_port: The port to use for discovery.
        timeout: The time to wait for a response in seconds.

    Returns:
        The proxy URL if found, otherwise None.
    """
    # Use urllib.parse to ensure there is a scheme.
    # Add '//' to treat inputs like 'example.com' as a network location.
    if "//" not in target_url:
        target_url = "//" + target_url

    # urlparse with a default scheme will add it if missing.
    parsed = urlparse(target_url, scheme="http")
    target_url = parsed.geturl()

    # Create a UDP socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    # Enable broadcasting
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)

    # Set a timeout for receiving
    sock.settimeout(timeout)

    # Prepare the message and broadcast address
    message = f"DISCOVER_REQ:{target_url}".encode("utf-8")
    broadcast_address = ("255.255.255.255", discovery_port)

    try:
        # Send the broadcast message
        sock.sendto(message, broadcast_address)
        logging.debug(f"Sent discovery request for {target_url}...")

        # Listen for a response
        while True:
            data, addr = sock.recvfrom(1024)
            response = data.decode("utf-8")
            if response.startswith("DISCOVER_RESP:"):
                proxy_url = response.replace("DISCOVER_RESP:", "", 1)
                logging.debug(f"Discovered proxy at {proxy_url} from {addr[0]}")
                return proxy_url
    except socket.timeout:
        # This is expected if no server responds
        logging.info("Discovery timed out. No proxy found.")
        return None
    finally:
        sock.close()

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: python3 {sys.argv[0]} <url_to_discover>", file=sys.stderr)
        sys.exit(1)

    url_to_discover = sys.argv[1]
    proxy_url_found = discover_proxy(url_to_discover)
    if proxy_url_found:
        # Print the proxy URL to stdout for scripting
        print(proxy_url_found, file=sys.stdout)
