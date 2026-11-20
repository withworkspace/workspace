#!/usr/bin/env bash
# A mission whose step has a check you decided against, its judge holding to the check and its orchestrator asking you.
#
# For seeing the way on from a Work step whose checks came round not all
# met, when sending it back would go against what you decided: the
# orchestrator can't pass such a step, and the planner can't change a step
# that's begun, so its question is the step's own, with passing it as it
# is among its answers.
#
#   scripts/try.sh up checks-unmet
#
# It adds the mission checks-unmet: a capsule demo/checks-unmet, and a
# plan whose step 1 checks:
#
#   1. `go test ./...` passes        (ws runs it)
#   2. the greeting comes from greeting() in main.go, and a VERSION
#      file holds the release's version   (in words: the first half is
#      true, and the second is what you decide against)
#
# Its crew c1 is a shell (no agent). As c1, the seed asks you whether to
# drop the VERSION file; as you, it answers "A" (drop it) on c1's page,
# which ws keeps as a decision of yours, with the question as its why. c1
# then hands over a bundle and reports done.
#
# No model is asked anything: marketing's stand-in for Claude Code plays
# the judge and the orchestrator from scripts (checks-unmet/agent), saying
# what they said the day this was found. The judge holds to check 2 as
# it's written (a real one is told your decision stands over it, and may
# not), and the orchestrator asks you. A stand-in has to be on the PATH of
# the try's daemon, so this seed goes first, or alone.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds"
slug="checks-unmet"

if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi

# The stand-in, before the try's first ws command starts its daemon.
mkdir -p "$dir/.try/bin" "$dir/.try/agent"
if [ ! -x "$dir/.try/bin/claude" ]; then
	cp "$from/marketing/stand-in/claude" "$dir/.try/bin/claude"
	chmod +x "$dir/.try/bin/claude"
fi
cp "$from/checks-unmet/agent"/*.md "$dir/.try/agent/"

# Crew here may be a shell: c1 is one, with no agent to pay for.
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi

"$ws" mission new "Checks unmet" --brief "## Goal
See the way on from a step whose check you decided against, once its judge has held to the check.

## Context
- Seeded by the workspace repo's scripts/seeds/checks-unmet.sh. Nothing is to change: c1 is a shell, and a stand-in plays the judge and the orchestrator.
" >/dev/null

"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
capsule="$dir/repos/demo/$slug"

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- The question at step 1 has been seen, and the step passed as it is.
## Stops early
- None.
## Steps
1. Work: **Version the greeting.** Give the release a version.
   - in: \`demo/$slug\`
   - check: \`go test ./...\` passes
   - check: the greeting comes from \`greeting()\` in \`main.go\`, and a \`VERSION\` file holds the release's version
2. Review: **Look at it.** What step 1 came to.
EOF

"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "You're a shell: nothing to do. A question is asked as you, a bundle handed over, and done reported, to run the step's checks." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done

# as <ws args...>: a ws command, as c1.
as() { (cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 "$@"); }

# c1 asks, and you answer on its page: the answer is typed into c1, a
# shell, which makes nothing of it, and ws keeps it as your decision.
as mission ask "A VERSION file would have to be kept in step with the tag by hand, and the release reads the tag. Options: (A, my recommendation) no VERSION file: the release's version is its tag, and nothing in the repo holds it; (B) add a VERSION file, as check 2 asks. A goes against check 2 as worded, so I need your call. Which one?" >/dev/null 2>&1

link="$("$ws" web --print)"
origin="${link%%/auth\?*}"
token="$(printf '%s' "$link" | sed -n 's/.*[?&]t=\([^&]*\).*/\1/p')"
curl -fsS -X POST -H "Authorization: Bearer $token" -H "Origin: $origin" -H 'Content-Type: application/json' \
	-d '{"text":"A"}' "$origin/api/missions/$slug/crew/c1/say" >/dev/null

# c1's own record of it, as crew keep one: its word for yours.
as mission decide "No VERSION file: the release's version is its tag" --why "User chose option A. This replaces step 1's check 2 wording about a VERSION file." >/dev/null 2>&1

(cd "$capsule" && { echo '$ go test ./...'; go test ./... 2>&1; echo '$ ls VERSION'; ls VERSION 2>&1 || true; } >checks.txt)
as mission bundle --set - >/dev/null <<'EOF'
## Summary
The greeting still comes from `greeting()`. As you chose, there is no VERSION file: the release's version is its tag.

## Try it
- try: `go test ./...`

## Checks
2. met: the greeting comes from `greeting()` in `main.go`. By your decision there is no VERSION file.
   - shows: `checks.txt` The tests passing, and no VERSION file

## Not verified
- Nothing.

## Noticed
- Nothing.
EOF

handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- No VERSION file, as the user chose (option A): the version of a release is its tag.\n## Learned\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' >"$handoff"
as mission done "No VERSION file, as the user chose: the release's version is its tag" --handoff "$handoff" >/dev/null

# The checks, the judge and the orchestrator are quick, with no model.
for _ in $(seq 1 100); do
	"$ws" mission show "$slug" 2>/dev/null | grep -q "Pass it" && break
	sleep 0.2
done

cat <<EOF
checks-unmet: step 1's checks came to 1 of 2 met. The judge (a stand-in)
held to check 2 as it's written, though you decided against a VERSION
file; the orchestrator (a stand-in) didn't send c1 back to go against
you, and asked you instead.

  /missions/$slug                    its question, as step 1's own: Verify b1, Pass it, Send it back, then its own answers
  /missions/$slug?tab=verify&b=b1    b1, on the Verify tab: approving it passes step 1
  $ws mission show $slug             the same, in the terminal
  $ws show -m $slug -f llm d1        your answer to c1, kept as your decision

Pass it, or approve b1, and step 1 passes. Answer in your own words (or
with one of the orchestrator's answers) and the orchestrator runs again
and accepts the step on what you said.
EOF
