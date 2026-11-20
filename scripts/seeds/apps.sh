#!/usr/bin/env bash
# An app of the workspace's own: API docs drawn from two repos' specs, with local names on.
#
# For looking at apps: a recipe in ws.toml with a page, a place in the
# sidebar showing its run in a frame, started when the page is opened and
# ended once nobody has it open; its two needs, each a repo's folder,
# pointed from the page's bar at the ground or a capsule.
#
#   scripts/try.sh up apps
#
# It writes [run.docs] into ws.toml (a python3 page standing in for
# Scalar, which reads openapi.json from demo's and records' folders), puts
# an openapi.json in each repo's ground, and lifts a capsule in each with
# one that's moved on: demo/orders-v2 and records/billing. Nothing's
# started: opening API docs in the sidebar starts it. It asks no agent
# anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds/apps"

if ! command -v python3 >/dev/null; then
	echo "the apps seed's page is python3, which isn't here" >&2
	exit 1
fi
if grep -q '^\[run\.docs\]' ws.toml; then
	echo "$dir already has the docs app" >&2
	exit 1
fi

# Names on: the app is framed at its own name.
if ! grep -q '^\[router\]' ws.local.toml; then
	printf '\n[router]\nenabled = true\n' >>ws.local.toml
fi

mkdir -p "$dir/apps"
cp "$from/docs.py" "$dir/apps/"

spec() { # spec <file> <title> <version> [<path> <verb> <summary>]...: an openapi.json
	python3 - "$@" <<'PY'
import json, sys
file, title, version, *ops = sys.argv[1:]
paths = {}
for i in range(0, len(ops), 3):
    paths.setdefault(ops[i], {})[ops[i + 1]] = {"summary": ops[i + 2]}
with open(file, "w") as f:
    json.dump({"openapi": "3.1.0", "info": {"title": title, "version": version}, "paths": paths}, f, indent=2)
PY
}

spec "$dir/repos/demo/.ground/openapi.json" "Orders BFF" "1.4" \
	/orders get "List orders" \
	/orders/{id} get "One order"
spec "$dir/repos/records/.ground/openapi.json" "Records" "2.0" \
	/records get "List records" \
	/records post "Add a record"

"$ws" lift demo orders-v2 >/dev/null
spec "$dir/repos/demo/orders-v2/openapi.json" "Orders BFF" "1.5" \
	/orders get "List orders" \
	/orders/{id} get "One order" \
	/orders/{id}/refund post "Refund an order" \
	/orders/{id} delete "Cancel an order"
"$ws" lift records billing >/dev/null
spec "$dir/repos/records/billing/openapi.json" "Records" "2.1" \
	/records get "List records" \
	/records post "Add a record" \
	/invoices get "List invoices"

cat >>ws.toml <<'TOML'

# API docs: the workspace's own, drawn from demo's and records' specs.
[run.docs]
command = "python3 apps/docs.py {port}"
ready = "/"
page = { title = "API docs", icon = "book" }

[run.docs.needs.demo]
repo = "demo"
env = "DEMO_DIR"

[run.docs.needs.records]
repo = "records"
env = "RECORDS_DIR"
TOML

cat <<EOF2
apps: an app of the workspace's own.

  /apps/docs                       API docs, in the sidebar: opening it starts it; the bar points demo and records
  /runs                            its run, and who it's for
  cd $dir
  $ws run point docs               what its needs are pointed at, and could be
  $ws run point docs demo=orders-v2
EOF2
