#!/usr/bin/env bash
# Automations you run yourself: actions on the fixture plugin's things, on a mission, and one on nothing in particular. Needs plugin-refs.
#
# For trying an automation's actions (when = action(...)) where you act on
# things:
#
#   scripts/try.sh up plugin-refs actions
#
# It adds three automations:
#   thing-actions.star    "Ask after it" (asks a question and how, first)
#                         and "Dust it" (only on a thing on the shelf: it
#                         acts, rewriting the thing's note through the
#                         plugin, once you've confirmed it), on the
#                         fixture's things (THING-1 to THING-3)
#   moved-notes.star      a thing moved (its state), its note rewritten to
#                         say where, without asking: granted here
#   mission-actions.star  "Note where it's at", on a mission under way
#   digest.star           "Write today's digest", on nothing: its page,
#                         and a notification that opens it
set -euo pipefail

ws="$TRY_WS"
here="$(cd "$(dirname "$0")" && pwd)/actions"

for f in "$here"/*.star; do
	"$ws" automation add "$f" >/dev/null
done
# Yours to say: moved-notes does it without asking.
"$ws" automation grant moved-notes frame-fixture:things >/dev/null

cat <<SAID
actions: thing-actions, mission-actions and digest, each an action of yours.

  Find (⌘K), under Do        Write today's digest
  /x/frame-fixture/things/2  Find's Here: Ask after it…, Dust it
  a row of THING-2, right click (a mission's brief, Find's rows)
  /missions/shelve-the-lantern   Find's Here, and its row's menu on /missions: Note where it's at
  /automations/thing-actions     its runs, each by you on what it was run on
  Dust it on THING-2            its card, in front of you to confirm; accepted, the note's rewritten
  Move to… on THING-3 (Find)    moved-notes rewrites its note without asking: /automations/moved-notes
SAID
