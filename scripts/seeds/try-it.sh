#!/usr/bin/env bash
# A bundle whose Try it names a recipe (ws run web): a button to start it, not a command to copy.
#
# For looking at Try it's start buttons on the Verify tab: a recipe a
# bundle's try: names, `ws run web` alone, started in the bundle's capsule
# with a button (and what it needs with it), then its link once it's up;
# a task one under a check, run with one; and a command that isn't a
# recipe, to copy as before.
#
#   scripts/try.sh up try-it
#
# It adds the mission try-it, with a capsule demo/try-it whose
# ws.repo.toml has web (a python3 page that needs records' api, mounted at
# /api, as the needs seed has it) and shots (a task that ends by itself),
# and gives records' ground the api recipe. Its crew c1 is a shell: the
# seed hands over its bundle b1, and reports done. It turns on [router]
# in ws.local.toml. It asks no agent anything but the judge, of nothing.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds/needs"
slug="try-it"

if ! command -v python3 >/dev/null; then
	echo "the try-it seed's servers are python3, which isn't here" >&2
	exit 1
fi
if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi
if ! grep -q '^\[router\]' ws.local.toml; then
	printf '\n[router]\nenabled = true\n' >>ws.local.toml
fi

"$ws" mission new "Try it" --brief "## Goal
See a bundle's Try it start what it names, with a button.

## Context
- Seeded by the workspace repo's scripts/seeds/try-it.sh. Nothing is to change: c1 is a shell, hands over a bundle, and reports done.
" >/dev/null
"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
capsule="$dir/repos/demo/$slug"

cp "$from/serve.py" "$from/index.html" "$capsule/"
cat >>"$capsule/ws.repo.toml" <<'TOML'

[run.service.web]
command = "python3 serve.py {port}"
ready = "/"
display_name = "Storefront"

[run.service.web.needs.api]
run = "records/api"
env = "API_URL"
mount = "/api"

[run.task.shots]
command = "sleep 3 && echo took 2 pictures"
TOML
(cd "$capsule" && git add -A && git -c user.name=seed -c user.email=seed@example.com commit -qm "web and shots recipes")

ground="$dir/repos/records/.ground"
if ! grep -q 'run.service.api' "$ground/ws.repo.toml" 2>/dev/null; then
	cp "$from/api.py" "$ground/"
	cat >>"$ground/ws.repo.toml" <<'TOML'

[run.service.api]
command = "python3 api.py {port}"
ready = "/api/hello"
TOML
fi

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- b1's Try it has been started from the Verify tab.
## Stops early
- None.
## Steps
1. Work: **Show the storefront.** Nothing to change: c1 hands over a bundle and reports done.
   - in: \`demo/$slug\`
   - check: the storefront shows what records' api says
2. Review: **Look at it.** b1, and its Try it.
EOF

"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "You're a shell: nothing to do. A bundle is handed over as you, and done reported." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done

(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission bundle --set - >/dev/null <<'EOF'
## Summary
The storefront shows what records' api says. Check 1 is met.

## Try it
- try: `ws run web`
- look for: the greeting from records' api, under Storefront
- try: `python3 serve.py 8099`

## Checks
1. met: The storefront shows the api's greeting, `hello from records`.
   - lines: `index.html:1-20`
   - try: `ws run shots`

## Not verified
- Nothing.
EOF
)

handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- Nothing: the step is for its bundle.\n## Learned\n- Nothing.\n## Environment\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' >"$handoff"
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "Nothing to change" --handoff "$handoff" >/dev/null)

cat <<EOF
try-it: b1's Try it names web (a recipe: a button), and its check 1 shots (a task).

  /missions/$slug?tab=verify&b=b1    Start web, then Open web once it's up; check 1's Run shots
EOF
