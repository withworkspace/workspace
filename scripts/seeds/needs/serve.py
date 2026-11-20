#!/usr/bin/env python3
"""The seed's web: a page, what it was started with, and a proxy of its own.

Asked for by its name, ws's router answers /api itself (the recipe's
mount) and this never sees it. Asked for at 127.0.0.1 and its port, there
is no router between, and /api is passed on to $API_URL here, as a dev
server's own proxy would.
"""
import json
import os
import sys
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

API = os.environ.get("API_URL", "")


class Web(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/env.json":
            self.reply(200, "application/json", json.dumps({"API_URL": API, "port": int(sys.argv[1])}).encode())
        elif self.path.startswith("/api/"):
            try:
                req = urllib.request.Request(API + self.path, headers={"X-Via": "web's own proxy, to $API_URL"})
                with urllib.request.urlopen(req, timeout=5) as r:
                    self.reply(r.status, r.headers.get("Content-Type", "application/json"), r.read())
            except Exception as e:  # the api isn't there
                self.reply(502, "application/json", json.dumps({"error": str(e), "API_URL": API}).encode())
        else:
            with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "index.html"), "rb") as f:
                self.reply(200, "text/html; charset=utf-8", f.read())

    def reply(self, status, kind, body):
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print("web:", fmt % args, flush=True)


print("web: listening on", sys.argv[1], "with API_URL", API or "(not set)", flush=True)
ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Web).serve_forever()
