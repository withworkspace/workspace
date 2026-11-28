#!/usr/bin/env bash
# A mission whose step has a slow check, with its checks running.
#
# For seeing what ws shows while it checks a step (the headline, ticking,
# with the check's last line; then the judge's turn), and what it logs
# (the step's Lately list, exec.log).
#
#   scripts/try.sh up slow-check
#
# It adds the mission slow-check: a capsule demo/slow-check holding
# slow-build.sh, which prints a line every 5s for a minute, and a plan
# whose step 1 checks:
#
#   1. `sh slow-build.sh` passes      (slow, with output)
#   2. `go test ./...` passes         (quick)
#   3. and 4. in words, for the judge (Claude on haiku, as the test
#      workspace has it); 4 is one the judge can't tell, so the step stays
#      at 1, and you can run the round again.
#
# Its crew c1 is a shell (no agent), and the seed reports done as c1,
# which has the workspace's daemon run the checks. Run the round again
# with the command it prints.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="slow-check"

if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi

# Crew here may be a shell: c1 is one, with no agent to pay for.
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi

"$ws" mission new "Slow check" --brief "## Goal
See what ws shows while it checks a step: a slow command, with output, then the judge.

## Context
- Seeded by the workspace repo's scripts/seeds/slow-check.sh. Nothing is to change: c1 is a shell, and reports done to run the checks.
" >/dev/null

"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
capsule="$dir/repos/demo/$slug"
cat >"$capsule/slow-build.sh" <<'SH'
#!/bin/sh
# A slow check, with output: a line every 5s, for a minute.
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
	echo "building part $i of 12"
	sleep 5
done
echo "all 12 parts built"
SH
git -C "$capsule" add slow-build.sh
git -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false -C "$capsule" commit -q -m "Add a slow build, for a slow check"

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- The slow check has been seen running, in the web UI and in the logs.
## Stops early
- None.
## Steps
1. Work: **Slow check.** Nothing to change: c1 reports done, and ws runs the step's checks, the first slow.
   - in: \`demo/$slug\`
   - check: \`sh slow-build.sh\` passes
   - check: \`go test ./...\` passes
   - check: main.go still prints a greeting
   - check: the user has watched this step's checks run in the web UI
2. Review: **Look at it.** What ws showed while it checked step 1.
EOF

"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "You're a shell: nothing to do. Report done to run the step's checks." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done

# What c1 hands off each time it reports done.
handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- Nothing: the step is for its slow check.\n## Learned\n- Nothing.\n## Environment\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' >"$handoff"
done_again="(cd '$capsule' && '$ws' WS_MISSION=$slug WS_CREW=c1 mission done 'Nothing to change' --handoff '$handoff')"
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "Nothing to change" --handoff "$handoff" >/dev/null)

cat <<EOF
slow-check: its checks are running. The slow one takes a minute, then go
test, then the judge.

  /missions/$slug             the mission's headline, ticking (in the web UI)
  $ws mission show $slug      the same, in the terminal
  $ws show -m $slug s1        step 1's Lately list: each check's start, end and result
  tail -f $dir/.ws/exec.log   each check's process, and the judge's

Run the round again (step 1 stays, as the judge can't tell of check 4;
the last round's question to you stays above the headline until you
answer it):

  $done_again
EOF
