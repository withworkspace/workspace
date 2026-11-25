#!/usr/bin/env bash
# A stream of events: capsules landed and burned, a mission started, a decision, an artifact promoted.
#
# For looking at what ws keeps of what it did (ws events), and so at what
# notifications will be made from:
#
#   scripts/try.sh up events
#
# It lands a capsule of records by hand, with no mission (capsule.landed
# and capsule.burned, by you); burns a capsule of demo it lifted for
# nothing; starts the mission tidy-the-notes, lands a capsule of records for
# it (a mission's landing, kept from its log), and records a decision on it
# (by you); then adds an artifact and promotes it. Run
# ws events there to see them, and ws events --follow while you land or
# promote something more. It asks no agent anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="tidy-the-notes"

if [ -d "$dir/.ws/missions/$slug" ]; then
	echo "$TRY_DIR already has the mission $slug" >&2
	exit 1
fi

commit() { # commit <capsule dir> <file> <subject>
	mkdir -p "$(dirname "$1/$2")"
	echo "# $3" >"$1/$2"
	git -C "$1" add -- "$2"
	git -C "$1" -c commit.gpgsign=false commit -q -m "$3"
}

# Landed by hand, with no mission.
"$ws" lift records loose-note >/dev/null 2>&1
commit "$dir/repos/records/loose-note" notes/loose.md "Add a loose note"
"$ws" land records loose-note >/dev/null 2>&1

# Lifted, and burned unused.
"$ws" lift demo spike >/dev/null 2>&1
"$ws" burn demo spike >/dev/null 2>&1

# A mission's: started, a capsule of it landed, and a decision.
"$ws" mission new "Tidy the notes" --brief "## Goal
The notes in records are tidy.

## Context
- Seeded by the workspace repo's scripts/seeds/events.sh.
" >/dev/null 2>&1
"$ws" WS_MISSION="$slug" lift records "$slug" >/dev/null 2>&1
commit "$dir/repos/records/$slug" notes/index.md "Index the notes"
"$ws" land records "$slug" >/dev/null 2>&1
"$ws" mission decide -m "$slug" "Keep one note per topic." --why "Easier to find." >/dev/null 2>&1

# An artifact, added and promoted.
page="$(mktemp -d)/page.html"
echo '<!doctype html><title>Notes layout</title><p>One note per topic.</p>' >"$page"
"$ws" artifact add "$page" --title "Notes layout" >/dev/null 2>&1 || true
"$ws" artifact promote a1 >/dev/null 2>&1 || true

echo "events: $("$ws" events | wc -l | tr -d ' ') kept; ws events lists them, ws events --follow shows more as they come"
