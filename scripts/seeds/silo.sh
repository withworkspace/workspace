#!/usr/bin/env bash
# A silo the daemon keeps: demo's, pointed at a capsule, with hooks.
#
# For looking at silos (ws silo): the daemon mirroring a capsule into
# .silo as its files change, with no ws silo watch open, re-pointing it
# when ws.local.toml says, and running its hooks.
#
#   scripts/try.sh up silo
#
# It lifts two capsules in demo, silo-a and silo-b, gives demo's ground a
# ws.repo.toml with an after_switch and an after_change hook (each adds a
# line to .silo/hooks.log), and points demo's silo at silo-a. Edit a file
# in silo-a and it's in repos/demo/.silo within a moment; ws silo point
# demo silo-b moves it. The Runs page shows how it stands. It asks no
# agent anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"

if [ -d "$dir/repos/demo/.silo" ]; then
	echo "$dir already has demo's silo" >&2
	exit 1
fi

"$ws" lift demo silo-a >/dev/null
"$ws" lift demo silo-b >/dev/null
echo "silo-a's own" >"$dir/repos/demo/silo-a/SILO.md"
echo "silo-b's own" >"$dir/repos/demo/silo-b/SILO.md"

cat >>"$dir/repos/demo/.ground/ws.repo.toml" <<'TOML'

[silo]
after_switch = "echo \"switched to $(cat SILO.md)\" >> hooks.log"
after_change = "echo changed >> hooks.log"
TOML

"$ws" silo point demo silo-a >/dev/null 2>&1

echo "silo: demo's silo is on silo-a, kept by the daemon (repos/demo/.silo; hooks.log there)"
