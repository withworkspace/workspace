#!/usr/bin/env bash
# A plugin whose things have names (THING-2), and a mission and a note that name them.
#
# For seeing a plugin's refs: a name found in what's written, its card,
# where it goes, what ws show says of it, its things found by what they
# say, and all of it gone when the plugin's removed.
#
#   scripts/try.sh up plugin-refs
#
# It adds the fixture plugin (testdata/plugins/frame-fixture), which
# registers how its things are named (THING-1 to THING-3 are there) and
# tells ws of the three, so they're found by what they say, a
# mission, shelve-the-lantern, whose brief names two of them, and a note in
# records that names them, one by where it is, beside some words that only
# look like names.
set -euo pipefail

ws="$TRY_WS"
slug="shelve-the-lantern"

"$ws" plugin add "$TRY_REPO/testdata/plugins/frame-fixture" --link --build >/dev/null
# Started, it registers its refs, and ws keeps them.
"$ws" frame-fixture poke >/dev/null

"$ws" mission new "Shelve the lantern" --brief "## Goal
THING-2 goes back on the shelf, beside THING-1.

## Context
- Seeded by the workspace repo's scripts/seeds/plugin-refs.sh.
" >/dev/null

# A document, where an address is read as the thing it's the place of.
"$ws" doc add note - >/dev/null <<NOTE
# Where the things are

THING-2 goes back on the shelf, beside THING-1.

The compass is at https://fixture.example/things/THING-3?from=the-note, and stays where it is.

THING-9 has a name and isn't there. THING-2-fix, docs/THING-2.md and UTF-8 aren't names at all.
NOTE

cat <<SAID
plugin-refs: the fixture plugin names its things THING-<n>, and a mission's
brief and a note name them.

  /missions/$slug   its goal: THING-2 and THING-1 are refs, each with a card
  /documents/records/notes/where-the-things-are.md
                    a click on THING-2 opens it in the fixture's app; the address is
                    THING-3; THING-9's card says there's none; the rest are words
  $ws show THING-2           what the plugin says it is, and what's said of it here
  $ws show THING-2 --full    with what it left out
  $ws show https://fixture.example/things/THING-3
  $ws show THING-9           the fixture has none
  $ws find lantern           found by what it says, as the plugin told ws of it
  Find (⌘K): lantern, or THING-2; THING-9, which ws wasn't told of, by its name alone
  $ws plugin list            names THING-…
  $ws plugin remove frame-fixture   and its names are words again
SAID
