#!/usr/bin/env bash
# A plugin that proposes (fixture things), and a conversation asking for some.
#
# For seeing a plugin's proposals: the agent asks the plugin how one's
# written (ws propose frame-fixture:things), proposes, and the card under
# its reply shows what the plugin says it'll do, item by item, to accept
# all or some. An anvil's too heavy for the shelf: accepting it fails,
# and stays to try again.
#
#   scripts/try.sh up plugin-proposals
set -euo pipefail

ws="$TRY_WS"

"$ws" plugin add "$TRY_REPO/testdata/plugins/frame-fixture" --link --build >/dev/null
# Started, it registers what it proposes, and agents are told.
"$ws" frame-fixture poke >/dev/null

"$ws" talk --print "Can you propose some fixture things for the shelf: a Teapot beside the Kettle (THING-1), a Tea cosy beside that teapot, and an Anvil. And change the Lantern's note to say it's been dusted." >/dev/null

cat <<SAID
plugin-proposals: the fixture plugin proposes things (frame-fixture:things), and
conversation w1 asks for some; its agent (haiku) answers in a minute or so.

  /conversations/w1   the card under its reply: three new things (one beside #1) and a note
                      changed (was → now); tick some, Accept; the Anvil fails and stays open;
                      Decline the rest, or ask for a change and see it revised (Revises N)
  $ws show w1         what's proposed, and what came of it
  $ws show THING-4    a thing made by accepting it, linked from w1
SAID
