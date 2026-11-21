#!/usr/bin/env bash
# Tasks: runs that end by themselves, kept for what they came to.
#
# For looking at the other type of run (ws run): a recipe under
# [run.task] has no port, runs to its end, and is kept: how it exited,
# how long it took, all it printed, the commit it ran at, the reports it
# wrote, and the runs of it before. A step's checks that are commands are
# kept the same way.
#
#   scripts/try.sh up tasks
#
# It adds the mission count-the-greetings, with a capsule in demo whose
# ws.repo.toml has:
#
#   test    go test with a coverage profile, kept as its coverage report
#   lint    gofmt, which fails: the seed leaves a file badly formatted
#   serve   a service, beside them (python3, if it's here)
#
# It runs test twice and lint once, so the Runs page has results, one of
# them with a run behind it. Then a shell stands in as crew and reports
# done, so ws runs step 1's checks: `ws run test`, which is the recipe's
# own run, a command, and one for the judge. It asks no agent anything
# but the judge.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="count-the-greetings"

if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi

"$ws" mission new "Count the greetings" --brief "## Goal
demo counts how many it has greeted.

## Context
- Seeded by the workspace repo's scripts/seeds/tasks.sh, for looking at tasks: runs that end by themselves. Nothing is to change: c1 is a shell, and reports done to run the checks.
" >/dev/null
"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null

capsule="$dir/repos/demo/$slug"
cat >>"$capsule/ws.repo.toml" <<'TOML'

[run.task.test]
command = "go test -coverprofile=coverage.out ./..."

[run.task.test.reports.coverage]
path = "coverage.out"
format = "gocover"

[run.task.lint]
command = "test -z \"$(gofmt -l . | tee /dev/stderr)\""
TOML
if command -v python3 >/dev/null; then
	cat >>"$capsule/ws.repo.toml" <<'TOML'

[run.service.serve]
command = "python3 -m http.server {port}"
ready = "/"
TOML
fi
# What lint finds.
printf 'package main\n\nfunc   count( ) int {\nreturn 0 }\n' >"$capsule/count.go"
printf 'coverage.out\n' >>"$capsule/.gitignore"
git -C "$capsule" add -A
git -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false -C "$capsule" commit -q -m "Count the greetings, badly formatted"

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- demo counts its greetings, tested.
## Stops early
- None.
## Steps
1. Work: **Count them.** Nothing to change: c1 reports done, and ws runs the step's checks.
   - in: \`demo/$slug\`
   - check: \`ws run test\` passes
   - check: \`go vet ./...\` passes
   - check: count.go is formatted as gofmt has it
2. Review: **Look at it.** What each check's run kept.
EOF

# Results to look at: test twice, lint once.
(cd "$capsule" && "$ws" run test >/dev/null 2>&1) || true
(cd "$capsule" && "$ws" run test >/dev/null 2>&1) || true
(cd "$capsule" && "$ws" run lint >/dev/null 2>&1) || true

"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "You're a shell: nothing to do. Report done to run the step's checks." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done
handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- Nothing: the step is for its checks.\n## Learned\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' >"$handoff"
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "Nothing to change" --handoff "$handoff" >/dev/null)

cat <<EOF
tasks: runs that end by themselves.

  /runs                            Results: test (run 3, with two behind it, its coverage report kept), lint (failed), and step 1's checks
  /missions/$slug                  Work tab: the capsule's runs, with Run test and Run lint; Verify and the plan: each command check goes to its run
  cd $capsule
  $ws run                          the recipes, with their types
  $ws run test                     prints what it prints, and exits as it did
  $ws run show test                what it came to, its report, the runs before it
  $ws run show test#1              an earlier one
  $ws run --task -- go vet ./...   a command as a task
  $ws run ls
EOF
