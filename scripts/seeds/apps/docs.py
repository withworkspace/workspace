"""API docs for the apps seed: an API reference drawn from two repos' specs.

It stands in for what a team runs for real (Scalar over TypeSpec's
OpenAPI): each spec is read from a folder ws hands over, DEMO_DIR and
RECORDS_DIR, which are whatever worktree each need is pointed at. Started
again on another, it draws that one's.

  python3 docs.py <port>
"""

import html
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer


def spec_section(name: str, folder: str) -> str:
    path = os.path.join(folder, "openapi.json")
    try:
        with open(path) as f:
            spec = json.load(f)
    except OSError:
        return f"<section><h2>{html.escape(name)}</h2><p class=dim>No openapi.json in {html.escape(folder)}.</p></section>"
    info = spec.get("info", {})
    rows = []
    for p, ops in spec.get("paths", {}).items():
        for verb, op in ops.items():
            rows.append(
                f"<tr><td><span class='verb {verb}'>{verb.upper()}</span></td><td><code>{html.escape(p)}</code></td><td>{html.escape(op.get('summary', ''))}</td></tr>"
            )
    where = os.path.basename(folder.rstrip("/"))
    return (
        f"<section><h2>{html.escape(info.get('title', name))} <small>v{html.escape(str(info.get('version', '?')))}</small></h2>"
        f"<p class=dim>From <code>{html.escape(where)}</code></p>"
        f"<table>{''.join(rows)}</table></section>"
    )


PAGE = """<!doctype html>
<meta charset=utf-8>
<title>API docs</title>
<style>
  body {{ font: 14px/1.5 system-ui, sans-serif; margin: 0; padding: 24px 32px; color: #1d1d1f; background: #fff; }}
  h1 {{ font-size: 20px; margin: 0 0 16px; }}
  h2 {{ font-size: 16px; margin: 24px 0 2px; }}
  small {{ color: #888; font-weight: 400; }}
  .dim {{ color: #888; margin: 0 0 8px; }}
  table {{ border-collapse: collapse; width: 100%; max-width: 720px; }}
  td {{ padding: 6px 8px; border-top: 1px solid #eee; vertical-align: top; }}
  code {{ font: 12.5px ui-monospace, monospace; }}
  .verb {{ font: 600 11px ui-monospace, monospace; padding: 2px 6px; border-radius: 4px; background: #eef; color: #335; }}
  .verb.post {{ background: #e8f6ec; color: #1d6b34; }}
  .verb.delete {{ background: #fbeaea; color: #8a2020; }}
</style>
<h1>API reference</h1>
{sections}
"""


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/", "/index.html"):
            self.send_error(404)
            return
        body = PAGE.format(
            sections=spec_section("BFF", os.environ["DEMO_DIR"]) + spec_section("Domain", os.environ["RECORDS_DIR"])
        ).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        sys.stderr.write("docs: " + fmt % args + "\n")


if __name__ == "__main__":
    port = int(sys.argv[1])
    print(f"docs: serving the specs of {os.environ['DEMO_DIR']} and {os.environ['RECORDS_DIR']} on {port}", flush=True)
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
