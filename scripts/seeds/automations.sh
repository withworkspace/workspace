#!/usr/bin/env bash
# Two automations, on: a sketch Brendan draws, and a decision changing in records; and Brendan landing both.
#
# For looking at automations (ws automation) and what they tell you:
#
#   scripts/try.sh up automations
#
# It adds drawing.star (notify me if Brendan pushes a new drawing) and
# doc-changes.star (tell me when a decision changes in records), then
# lands Brendan's sketch and decision on records. The daemon hears the
# landing, and runs both: two notifications, from Automations, in the
# feed (the bell; ws notifications). ws automation list and ws automation
# runs say what each did. It asks no agent anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
here="$(cd "$(dirname "$0")" && pwd)/automations"

if [ -d "$dir/.ws/automations" ]; then
	echo "$TRY_DIR has automations already" >&2
	exit 1
fi

# Where the grounds are now isn't news.
"$ws" events hear >/dev/null 2>&1 || true
"$ws" automation add "$here/drawing.star" >/dev/null
"$ws" automation add "$here/doc-changes.star" >/dev/null

"$ws" lift records release-flow >/dev/null 2>&1
c="$dir/repos/records/release-flow"
mkdir -p "$c/sketches/release-flow" "$c/decisions"
printf 'kind = "sketch"\ntitle = "Release flow"\nmain = "release-flow.excalidraw"\n' >"$c/sketches/release-flow/thing.toml"
printf '{"elements":[]}\n' >"$c/sketches/release-flow/release-flow.excalidraw"
printf '# Release from main, weekly\n\nA release is cut from main every Tuesday.\n' >"$c/decisions/release-weekly.md"
git -C "$c" add -A
GIT_AUTHOR_NAME="Brendan Ash" GIT_AUTHOR_EMAIL=brendan@example.com GIT_COMMITTER_NAME="Brendan Ash" GIT_COMMITTER_EMAIL=brendan@example.com \
	git -C "$c" -c commit.gpgsign=false commit -q -m "Draw the release flow, and decide we release weekly"
(cd "$c" && "$ws" land >/dev/null 2>&1)

# What the daemon would hear by itself, now (and the automations run once
# the daemon's up).
"$ws" events hear >/dev/null 2>&1 || true

cat <<OUT
automations: drawing and doc-changes, on; Brendan landed a sketch and a decision on records.

  ws automation list      each, and how its last run went
  ws automation runs      what each did
  ws notifications        what they told you (the bell, in the web UI)
OUT
