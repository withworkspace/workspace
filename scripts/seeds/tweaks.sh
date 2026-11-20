#!/usr/bin/env bash
# An artifact whose page declares tweaks, in two versions.
#
# A prototype of a mission's plan list, by you. Its page says what can be
# changed about it (ws.tweaks: a density, how many steps are shown, pips,
# a radius, a colour, a heading) and ws draws the tray beside its frame.
# The second version opens as what was picked on the first, and its
# tweaks aren't the same ones: a state belongs to the version it was set
# on. It asks no agent anything, so it's the same every time.
#
#   scripts/try.sh up tweaks
set -euo pipefail

ws="$TRY_WS"
from="$TRY_REPO/scripts/seeds/tweaks"

# Its ref is what ws says it is: another seed may have added one first.
ref="$("$ws" artifact add "$from/plan-v1.html" --title "Plan list" \
	--note "A long plan in a mission's details: how much of it to show, and how tight. Its tweaks say what's open." 2>/dev/null)"
id="${ref%%.*}"
"$ws" artifact add "$from/plan-v2.html" --to "$id" \
	--note "Opens as what was picked on v1: compact, a window of 2 + 3, pips. Still open: the window's size, and the current step opened." >/dev/null

cat <<EOF2
tweaks: an artifact whose page declares tweaks.

  /artifacts/$id/v1                                  the first take: seven tweaks, three presets
  /artifacts/$id/v1?t.density=compact&t.pips=on      the same, as the address sets it
  /artifacts/$id                                     the second: other tweaks, opening as what was picked
  /artifacts/$id/compare?v=1,1&b.density=compact     v1 beside itself: as it opens, and compact
EOF2
