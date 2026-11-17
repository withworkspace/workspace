#!/usr/bin/env bash
# A mission ready to start: a brief, a capsule and a plan, and no crew yet.
#
# The plainest thing a try can hold, and the seed to copy for another: it
# asks no agent anything, so it's the same every time.
#
#   scripts/try.sh up mission
#
# It adds the mission greet-by-name: a capsule demo/greet-by-name, as main
# has it, and a plan of three steps (Work, Review, Open Pull Request). The
# mission is at step 1, in your court, with crew to launch.
set -euo pipefail

ws="$TRY_WS"
slug="greet-by-name"

if [ -d ".ws/missions/$slug" ]; then
	echo "$TRY_DIR already has the mission $slug" >&2
	exit 1
fi

# With a brief, it's ready straight away: without one it waits for a
# briefing agent.
"$ws" mission new "Greet by name" --brief "## Goal
demo greets whoever it's told to: \`go run . Ada\` prints hello, Ada.

## Context
- Seeded by the workspace repo's scripts/seeds/mission.sh.
" >/dev/null

# A capsule lifted for a mission is the mission's.
"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null

# A plan's quotes are checked against the process, so this one has none.
"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- \`go run . Ada\` prints hello, Ada, with a test, and its pull request is open.
## Stops early
- None.
## Steps
1. Work: **Greet by name.** greeting takes a name, and falls back to world.
   - in: \`demo/$slug\`
   - check: \`go test ./...\` passes
2. Review: **Read it.** The change, before its pull request.
3. Open Pull Request: **Open it.** From \`demo/$slug\`.
EOF

cat <<EOF
mission: $slug is ready, at step 1 of 3, with nobody started.

  /missions/$slug          its page (in the web UI): Launch crew starts Claude on haiku
  $ws mission show $slug   the same, in the terminal
  $TRY_DIR/repos/demo/$slug   its capsule

For crew that's no agent, scripts/seeds/slow-check.sh allows a shell as
crew, dispatches one, and reports done as it.
EOF
