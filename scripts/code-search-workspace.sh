#!/usr/bin/env bash
# Builds a throwaway workspace for trying code search by hand: the test
# workspace (scripts/test-workspace.sh), plus a capsule of demo that
# changes a file and adds one, so Find's % scope has something of a
# capsule's to show in place of .ground's copy.
#
#   scripts/code-search-workspace.sh [dir]     (default: ~/Projects/ws-code-search)
#
# WS is the ws to run (default: this checkout's bin/workspace, which `just
# build` makes, web UI included). Never point it at the workspace you work
# in: everything here is made to be thrown away. Its ws lift starts the
# workspace's daemon: stop it (cd <dir> && ws daemon stop) before you
# delete the folder and run this again for a fresh one, or ws web finds a
# daemon left over from the old one.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
dir="${1:-$HOME/Projects/ws-code-search}"
ws="${WS:-$here/bin/workspace}"
[ -x "$ws" ] || { echo "no ws at $ws: run just build, or set WS" >&2; exit 1; }

"$here/scripts/test-workspace.sh" "$dir" >/dev/null
dir="$(cd "$dir" && pwd)"
cd "$dir"

git_seed() { git -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false "$@"; }

# A capsule of demo, as crew would have it: main.go changed (committed),
# a new file added (committed), and one not committed yet. Its other files
# are main's, and code search across the workspace doesn't list them.
"$ws" lift demo search-demo >/dev/null 2>&1
capsule="$dir/repos/demo/search-demo"
cat >>"$capsule/main.go" <<'EOF'

// greetLoudly greets the way the capsule changed it: only this capsule's
// copy of main.go has it.
func greetLoudly(name string) string { return "HELLO, " + name + "!" }
EOF
cat >"$capsule/loud.go" <<'EOF'
package main

// loudness is new in the capsule: greet loudly, in a file main doesn't have.
const loudness = 11
EOF
# A long markdown file with a hit far down it: a hit at a line opens it as
# text, scrolled to the line, not rendered at the top.
{
	echo "# A long guide"
	for i in $(seq 2 239); do echo "Line $i of the guide, about nothing much."; done
	echo "Deep down: greet them here, at line 240."
	for i in $(seq 241 300); do echo "Line $i of the guide, about nothing much."; done
} >"$capsule/guide.md"
# Enough lines that a single letter is capped.
for i in $(seq 1 1000); do echo "line $i of a long file the capsule added"; done >"$capsule/long.txt"
git_seed -C "$capsule" add -A
git_seed -C "$capsule" commit -q -m "Say hello loudly"
cat >"$capsule/notes.txt" <<'EOF'
Greet them all: not committed yet, and code search finds what's untracked too.
EOF

cat <<EOF
Code search workspace ready: $dir

  cd $dir && $ws web

Then, in the web UI:
  - Find (mod+K), type %greet: demo's .ground main_test.go (the capsule
    didn't change it), records' notes and ADR, and from search-demo its
    main.go (in place of .ground's: main.go is listed once), loud.go and
    notes.txt (not committed). Not search-demo's main_test.go or go.mod.
  - %greet them here: guide.md:240, a long markdown file. Choosing it opens
    the file as text at line 240, marked (so does Find's /guide.md:240).
  - %e: a single letter, capped at 200 matches, and Find says so.
  - Files › demo (/files/demo, or a file in it): mod+shift+F opens Find in %
    tagged "repo: demo"; Backspace on the empty input clears the tag and
    the search widens to the workspace (records' hits come back).
  - Files › demo › search-demo (/files/demo/search-demo): mod+shift+F opens
    the explorer's own search, over all of the capsule, unchanged files
    too (greet: main_test.go is there now).
  - With Find or the explorer's search open, mod+shift+F does nothing.
  - Choose any hit: the file opens at its line.
EOF
