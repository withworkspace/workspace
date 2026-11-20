#!/usr/bin/env bash
# A mission being briefed, after marketing: its planner has asked, and proposed the brief twice, the second time with its plan.
#
# What a mission's Brief tab is for, to look at: the brief and the plan as
# the planner's second draft has them (its changes since the first marked
# in the brief, to accept or decline), the plan's steps beside them, and
# the conversation that got there. It follows the marketing seed, whose
# stand-in for Claude Code plays the planner, and briefs the mission that
# seed leaves to brief:
#
#   scripts/try.sh up marketing briefing
#
# The planner's three answers are scripts in scripts/seeds/briefing/agent/
# (the stand-in says how one's written): it reads round the request and
# asks three things; proposes the brief; then, asked for a change,
# proposes it again with its plan. What it proposes is ws's own, so the
# plan is one ws read and held to Sorrel's process.
#
# Confirming it there starts the mission (crew are the stand-in's, and
# wait); so it's a try's last seed, or looked at before one that needs
# the mission as it was.
set -euo pipefail

for tool in curl node; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "briefing needs $tool" >&2
		exit 1
	}
done

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds/briefing"
slug="archive-invoices-older-than-seven-years"

if [ ! -x "$dir/.try/bin/claude" ] || ! "$ws" mission show "$slug" >/dev/null 2>&1; then
	echo "briefing follows marketing, which has the mission it briefs and the planner's stand-in: scripts/try.sh up marketing briefing" >&2
	exit 1
fi

cp "$from"/agent/*.md "$dir/.try/agent/"

# The web UI's API, as marketing uses it: briefing is what its page does.
link="$("$ws" web --print)"
origin="${link%%/auth\?*}"
token="$(printf '%s' "$link" | sed -n 's/.*[?&]t=\([^&]*\).*/\1/p')"
api() { curl -fsS -H "Authorization: Bearer $token" -H "Origin: $origin" -H 'Content-Type: application/json' "$@"; }

# answered <n>: wait until the planner has answered n times, and isn't
# answering. An answer ws wrote for it (it couldn't) is the seed's to fix.
answered() {
	for _ in $(seq 1 150); do
		if api "$origin/api/missions/$slug/conversations/t1" | node -e '
			let s = "";
			process.stdin.on("data", (d) => (s += d)).on("end", () => {
				const t = JSON.parse(s);
				const failed = t.turns.find((x) => x.by === "ws");
				if (failed) {
					console.error(`the planner: ${failed.body}`);
					process.exit(2);
				}
				process.exit(!t.answering && t.turns.filter((x) => x.by === "agent").length >= Number(process.argv[1]) ? 0 : 1);
			});
		' "$1"; then
			return 0
		elif [ $? -eq 2 ]; then
			return 1
		fi
		sleep 0.2
	done
	echo "$slug: its planner never answered a turn $1" >&2
	return 1
}

# say <words>: yours, in its planner's conversation.
say() { api -X POST -d "$(node -e 'console.log(JSON.stringify({ body: process.argv[1] }))' "$1")" "$origin/api/missions/$slug/conversations/t1/say" >/dev/null; }

api -X POST -d '{"verb":"brief"}' "$origin/api/missions/$slug/act" >/dev/null
answered 1
say "Issue date, yes. Leave the ones still owed alone. No deleting."
answered 2
say "Count it in tax years: seven years after the end of the tax year it was issued in. And a studio should be able to bring one back. Then write the plan."
answered 3

cat <<EOF
briefing: a mission whose planner has proposed its brief twice.

  /missions/$slug?tab=brief   the second draft, its changes since the first to read, with its plan

Answer it, change the brief or the plan in place, or confirm: the mission
starts, and its crew (the stand-in's) wait.
EOF
