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
  Find (⌘K): compass, or THING-2, drawn beside the list as the plugin's pane says, from
                    what ws was told; THING-4, which ws wasn't told of, by its name alone
  Find (⌘K): on a thing's row, act on it: Move to…, or Start a mission about it;
                    under Do, the plugin's own: Dust the shelf
  $ws mission new "Polish it" --about THING-2    a mission about it: its page says so,
                    and $ws show THING-2 says the mission's about it
  $ws show THING-3           its links: part of THING-2, which says THING-3 is in it
  $ws frame-fixture said THING-2   the same, as ws answers a plugin that asks (show): what
                    it links to and what's said of it, as data, each with its page
  $ws frame-fixture talk "Let's shelve THING-2"   a conversation ws starts for a plugin that asks
                    (conversation/start): it says where it came from, its agent answers, and
                    $ws show THING-2 says at once that the conversation mentions it
  /x/frame-fixture/          the fixture's own page: under "Names in the app's own words",
                    THING-2, the mission and the note are refs there too, found by ws
                    for the app (its panel counts each ws:refs-changed)
  $ws plugin config frame-fixture shelves.top.label=Top   config in a table: a row's field set by
                    its path; $ws plugin config frame-fixture lists the rows, --ask asks a row
                    at a time, and $ws frame-fixture shelves prints them as the plugin was given them
  $ws plugin list            names THING-…
  $ws plugin remove frame-fixture   and its names are words again
SAID
