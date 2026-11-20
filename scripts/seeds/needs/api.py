#!/usr/bin/env python3
"""The seed's api: answers /api/hello with who it is and what it was asked as."""
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Api(BaseHTTPRequestHandler):
    def do_GET(self):
        if not self.path.startswith("/api/"):
            self.send_error(404, "the api answers below /api")
            return
        body = json.dumps(
            {
                "hello": "from the api",
                "runs_in": os.getcwd(),
                "port": int(sys.argv[1]),
                "asked_for": self.path,
                "asked_as": self.headers.get("Host", ""),
                "by_way_of": self.headers.get("X-Via", "ws's router: the mount at /api"),
            }
        ).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print("api:", fmt % args, flush=True)


print("api: listening on", sys.argv[1], flush=True)
ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Api).serve_forever()
