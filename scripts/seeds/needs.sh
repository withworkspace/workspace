#!/usr/bin/env bash
# A run that needs another: demo's web, which needs records' api, with local names on.
#
# For looking at needs (ws run): a recipe that says what it needs running
# first, ws starting that where it's nearest, one run of it shared by
# everything that needs it, and its mount below the run's own name.
#
#   scripts/try.sh up needs
#
# It adds the mission a-page-that-needs-an-api, with a capsule in demo
# whose ws.repo.toml has:
#
#   web     a page (python3), which needs api: records' api recipe, handed
#           over as $API_URL and mounted at /api
#   admin   another, which needs the same api
#
# and gives records' ground the api recipe. Nothing's started: Run web on
# the mission's Work tab, or ws run web in the capsule, starts both. It
# turns on [router] in ws.local.toml, as a mount is the router's to
# serve. It asks no agent anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds/needs"
slug="a-page-that-needs-an-api"

if ! command -v python3 >/dev/null; then
	echo "the needs seed's servers are python3, which isn't here" >&2
	exit 1
fi
if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi

# Names on: the mount answers below the run's own name.
if ! grep -q '^\[router\]' ws.local.toml; then
	printf '\n[router]\nenabled = true\n' >>ws.local.toml
fi

"$ws" mission new "A page that needs an api" --brief "## Goal
demo's page shows what records' api says.

## Context
- Seeded by the workspace repo's scripts/seeds/needs.sh, for looking at runs that need other runs.
" >/dev/null
"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null

capsule="$dir/repos/demo/$slug"
cp "$from/serve.py" "$from/index.html" "$capsule/"
cat >>"$capsule/ws.repo.toml" <<'TOML'

[run.service.web]
command = "python3 serve.py {port}"
ready = "/"

[run.service.web.needs.api]
run = "records/api"   # records' api recipe: its ground's, as the mission has no capsule there
env = "API_URL"       # its address, in web's environment
mount = "/api"        # and below web's own name, through ws's router

[run.service.admin]
command = "python3 serve.py {port}"
ready = "/"

[run.service.admin.needs.api]
run = "records/api"
env = "API_URL"
TOML

ground="$dir/repos/records/.ground"
cp "$from/api.py" "$ground/"
cat >>"$ground/ws.repo.toml" <<'TOML'

[run.service.api]
command = "python3 api.py {port}"
ready = "/api/hello"
TOML

cat <<EOF2
needs: a run that needs another.

  /missions/$slug                  Work tab: Run web starts records' api first, then web; the page it links to asks /api of its own name
  /runs                            both runs: what web needs, and what the api is needed by
  cd $capsule
  $ws run web                      the same, from a terminal; ws run admin then shares the api
  $ws run stop web admin           the api ends with the last of them
  $ws run --with api=https://example.com web    an address in place of a run
EOF2
