import argparse
import base64
import json
import logging
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional

class SiteInspector:
    """
    SiteInspector helps query and inspect captured HTTP/HTTPS traffic
    from the vpn-share-tool debug server by connecting directly to a specified address.
    """
    def __init__(self, addr: str = "http://127.0.0.1:8000", session_id: str = "live_session"):
        """
        Initialize the SiteInspector with a target server address.
        :param addr: Server address, e.g. "127.0.0.1:8000" or "http://localhost:8000"
        :param session_id: Debug session ID (defaults to "live_session")
        """
        self.session_id = session_id
        if not addr.startswith("http://") and not addr.startswith("https://"):
            self.api_url = f"http://{addr.rstrip('/')}"
        else:
            self.api_url = addr.rstrip('/')

        logging.debug(f"SiteInspector initialized with API URL: {self.api_url}, Session: {self.session_id}")

    def _request(self, path: str, method: str = "GET", data: Optional[bytes] = None, headers: Optional[Dict[str, str]] = None) -> Any:
        url = f"{self.api_url}{path}"
        req = urllib.request.Request(url, method=method, data=data, headers=headers or {})
        try:
            with urllib.request.urlopen(req) as resp:
                if resp.status in (200, 201, 204):
                    body = resp.read()
                    if body:
                        return json.loads(body)
                    return None
                raise RuntimeError(f"HTTP request failed with status {resp.status}")
        except urllib.error.HTTPError as e:
            raise RuntimeError(f"HTTP request failed: {e.code} {e.reason}")
        except Exception as e:
            raise RuntimeError(f"Connection to API server at {self.api_url} failed: {e}")

    def get_requests(self, search: Optional[str] = None, hide_errors: bool = False, types: Optional[str] = None, page: int = 1, limit: int = 50) -> Dict[str, Any]:
        """
        Get captured request summaries.
        :param search: Substring to filter requests by URL
        :param hide_errors: Hide requests with status >= 400
        :param types: Comma-separated list of types (XHR, DOC, JS, CSS, IMG, OTHER)
        :param page: Page number (1-indexed)
        :param limit: Number of requests per page
        """
        params = {
            "page": str(page),
            "limit": str(limit),
            "hide_errors": "true" if hide_errors else "false"
        }
        if search:
            params["search"] = search
        if types:
            params["types"] = types

        query_str = urllib.parse.urlencode(params)
        path = f"/debug/sessions/{self.session_id}/requests?{query_str}"
        return self._request(path)

    def get_xhr_requests(self, search: Optional[str] = None, page: int = 1, limit: int = 50) -> List[Dict[str, Any]]:
        """
        Get only XHR (API/JSON) requests.
        """
        res = self.get_requests(search=search, types="XHR", page=page, limit=limit)
        return res.get("requests", [])

    def get_doc_requests(self, search: Optional[str] = None, page: int = 1, limit: int = 50) -> List[Dict[str, Any]]:
        """
        Get only HTML/Document requests.
        """
        res = self.get_requests(search=search, types="DOC", page=page, limit=limit)
        return res.get("requests", [])

    def get_details(self, request_id: int) -> Dict[str, Any]:
        """
        Get the full details (including body and headers) of a single request.
        """
        return self._request(f"/api/debug/requests/{self.session_id}/{request_id}")

    def get_response_body(self, request_id: int) -> bytes:
        """
        Fetch a request's full response body and decode it if Base64 encoded.
        """
        details = self.get_details(request_id)
        body_str = details.get("response_body", "")
        is_base64 = details.get("is_base64", False)

        if is_base64:
            return base64.b64decode(body_str)
        return body_str.encode("utf-8")

    def save_response_body(self, request_id: int, output_path: str) -> None:
        """
        Fetch a request's full response body, decode if needed, and write to output_path.
        """
        data = self.get_response_body(request_id)
        os.makedirs(os.path.dirname(os.path.abspath(output_path)), exist_ok=True)
        with open(output_path, "wb") as f:
            f.write(data)

    def save_doc_response(self, url_pattern: str, output_path: str) -> bool:
        """
        Find the most recent HTML/Document request matching url_pattern and save its response body to output_path.
        """
        requests = self.get_doc_requests(search=url_pattern, page=1, limit=10)
        if not requests:
            return False
        latest_req = requests[0]
        self.save_response_body(latest_req["id"], output_path)
        return True

    def save_xhr_response(self, url_pattern: str, output_path: str) -> bool:
        """
        Find the most recent XHR request matching url_pattern and save its response body to output_path.
        """
        requests = self.get_xhr_requests(search=url_pattern, page=1, limit=10)
        if not requests:
            return False
        latest_req = requests[0]
        self.save_response_body(latest_req["id"], output_path)
        return True

    def clear_live_session(self) -> None:
        """
        Clear non-essential requests from the live session.
        """
        self._request("/debug/clear-live", method="POST")

def main():
    parser = argparse.ArgumentParser(description="Inspect captured traffic from vpn-share-tool debug server.")
    parser.add_argument("--addr", "-a", default="127.0.0.1:8000", help="API server address (default: 127.0.0.1:8000)")
    parser.add_argument("--session", "-s", default="live_session", help="Session ID (default: live_session)")

    subparsers = parser.add_subparsers(dest="command", required=True)

    # list-xhr command
    xhr_parser = subparsers.add_parser("list-xhr", help="List captured XHR requests")
    xhr_parser.add_argument("search", nargs="?", default=None, help="Search pattern for URL")

    # list-doc command
    doc_parser = subparsers.add_parser("list-doc", help="List captured Document requests")
    doc_parser.add_argument("search", nargs="?", default=None, help="Search pattern for URL")

    # save-xhr command
    save_xhr_parser = subparsers.add_parser("save-xhr", help="Save the response body of latest matching XHR request")
    save_xhr_parser.add_argument("pattern", help="Search pattern for URL")
    save_xhr_parser.add_argument("output", help="Output file path")

    # save-doc command
    save_doc_parser = subparsers.add_parser("save-doc", help="Save the response body of latest matching Document request")
    save_doc_parser.add_argument("pattern", help="Search pattern for URL")
    save_doc_parser.add_argument("output", help="Output file path")

    # clear command
    subparsers.add_parser("clear", help="Clear non-essential requests from live session")

    args = parser.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")

    inspector = SiteInspector(addr=args.addr, session_id=args.session)

    try:
        if args.command == "list-xhr":
            reqs = inspector.get_xhr_requests(search=args.search)
            print(f"Found {len(reqs)} XHR requests:")
            for r in reqs:
                print(f"  [{r['response_status']}] {r['method']} {r['url']} (ID: {r['id']})")

        elif args.command == "list-doc":
            reqs = inspector.get_doc_requests(search=args.search)
            print(f"Found {len(reqs)} Document requests:")
            for r in reqs:
                print(f"  [{r['response_status']}] {r['method']} {r['url']} (ID: {r['id']})")

        elif args.command == "save-xhr":
            if inspector.save_xhr_response(args.pattern, args.output):
                print(f"Saved latest matching XHR response to {args.output}")
            else:
                print(f"No matching XHR request found for pattern '{args.pattern}'")
                sys.exit(2)

        elif args.command == "save-doc":
            if inspector.save_doc_response(args.pattern, args.output):
                print(f"Saved latest matching Document response to {args.output}")
            else:
                print(f"No matching Document request found for pattern '{args.pattern}'")
                sys.exit(2)

        elif args.command == "clear":
            inspector.clear_live_session()
            print("Cleared non-essential requests from live session.")

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
