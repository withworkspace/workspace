#!/usr/bin/env bash
# A mission whose step's checks are yours to say, its crew's bundle handed over.
#
# For seeing what ws asks you when the judge can't tell of a check that
# crew's bundle shows: the question on the mission's page, its way to the
# bundle, and the bundle on the Verify tab, where approving it passes the
# step and saying what's not right sends it back.
#
#   scripts/try.sh up checks-yours
#
# It adds the mission checks-yours: a capsule demo/checks-yours, and a
# plan whose step 1 checks:
#
#   1. `go test ./...` passes        (ws runs it)
#   2. in words, for the judge (Claude on haiku, as the test workspace has
#      it): one it can't tell, which c1's bundle says is met, with what
#      the run printed.
#
# Its crew c1 is a shell (no agent): the seed hands over a bundle as c1,
# then reports done, which has the workspace's daemon run the checks. The
# judge takes a few seconds; then step 1 is yours.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="checks-yours"

if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi

# Crew here may be a shell: c1 is one, with no agent to pay for.
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi

"$ws" mission new "Checks yours" --brief "## Goal
See what ws asks when a step's checks are yours to say, and crew's bundle shows them.

## Context
- Seeded by the workspace repo's scripts/seeds/checks-yours.sh. Nothing is to change: c1 is a shell, hands over a bundle, and reports done to run the checks.
" >/dev/null

"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
capsule="$dir/repos/demo/$slug"

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- The question ws asks of step 1's checks has been seen, and its bundle looked at.
## Stops early
- None.
## Steps
1. Work: **Greet.** Nothing to change: c1 hands over a bundle and reports done, and ws runs the step's checks.
   - in: \`demo/$slug\`
   - check: \`go test ./...\` passes
   - check: the user has seen the greeting printed in a terminal of their own
2. Review: **Look at it.** What ws asked of step 1.
EOF

"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "You're a shell: nothing to do. A bundle is handed over as you, and done reported, to run the step's checks." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done

# What c1 saw: the greeting, as the run printed it. It's proof, not work,
# so it isn't committed.
(cd "$capsule" && { echo '$ go run .'; go run . 2>&1; } >greeting.txt)
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission bundle --set - >/dev/null <<'EOF'
## Summary
Nothing changed: the greeting prints as main has it. The run's output is kept, as c1 saw it in its own terminal.

## Try it
- try: `go run .`
- look: it prints a greeting

## Checks
2. met: the greeting printed in c1's terminal, not yours: what it printed is kept.
   - point: `go run .`: printed `hello`.
   - point: `go build && ./demo`: printed `hello`.
   - shows: `greeting.txt` What `go run .` printed

## Not verified
- The greeting in a terminal of your own: only you can say.

## Noticed
- `go run .` takes a second or two the first time, while it builds.
EOF
)

handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- Nothing: the step is for what ws asks of its checks.\n## Learned\n- Nothing.\n## Environment\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' >"$handoff"
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "Nothing to change" --handoff "$handoff" >/dev/null)

cat <<EOF
checks-yours: its checks are running: go test, then the judge, which
takes a few seconds. Then step 1 is yours: the judge can't tell of its
second check, and c1's bundle b1 says it's met.

  /missions/$slug                    what ws asks, with its way to b1 (in the web UI)
  /missions/$slug?tab=verify&b=b1    b1, on the Verify tab: approve it, or say what's not right
  $ws mission show $slug             the same question, in the terminal

Approving b1 passes step 1; saying what's not right of a check sends it
back to c1 (a shell: the page offers to launch crew in its place).
EOF
