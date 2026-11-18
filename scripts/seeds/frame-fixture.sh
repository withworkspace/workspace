#!/usr/bin/env bash
# The fixture plugin, whose app runs in the page ws serves for it (the shell).
#
# For seeing a plugin's app in the web UI: its first paint in ws's theme,
# a deep link, the address and the title following the app, the theme and
# the accent changing under it, ws:changed, and ws refusing a page it has
# no frame protocol version in common with.
#
#   scripts/try.sh up frame-fixture
#
# It adds the plugin in testdata/plugins/frame-fixture, linked and built
# (its README says what it is). In the web UI:
#
#   /x/frame-fixture/             its first screen; "Frame fixture" in the sidebar
#   /x/frame-fixture/things/2     a deep link, loaded fresh
#   /x/frame-fixture/?speaks=2    the app says it speaks only frame protocol 2: ws refuses it
#
# and `<dir>/.try/ws frame-fixture poke` tells the open app that what it
# shows is stale: its "ws:changed received" goes up. `<dir>/.try/ws
# frame-fixture looking` prints who ws last told it is looking: its app
# while that's on screen, ws alone on another page, and nobody a few
# seconds after the tab's put behind another.
set -euo pipefail

ws="$TRY_WS"

"$ws" plugin add "$TRY_REPO/testdata/plugins/frame-fixture" --link --build

cat <<SAID

The fixture plugin is added, linked to testdata/plugins/frame-fixture: after
a change there, adding it again builds it and runs what was built:

  $ws plugin add $TRY_REPO/testdata/plugins/frame-fixture --link --build

In the web UI:

  /x/frame-fixture/             its first screen
  /x/frame-fixture/things/2     a deep link
  /x/frame-fixture/?speaks=2    ws refuses it: "Update ws"

  $ws frame-fixture poke        the open app is told (ws:changed)
  $ws frame-fixture looking     who ws last said is looking (view/looking)
SAID
