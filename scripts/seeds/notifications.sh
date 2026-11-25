#!/usr/bin/env bash
# A feed of notifications: missions started for you, a pull request merged, a plugin's update, an upgrade.
#
# For looking at the feed of what happened (Notifications in the sidebar,
# its page, ws notifications), beside what needs you:
#
#   scripts/try.sh up notifications
#
# It has crew c1 and the orchestrator each start a mission, as they do for
# something they notice (real notifications, made from what they did), and
# adds by hand what a try can't make happen: a pull request this ws opened
# merging, a mission ending itself once it had, a plugin's update, an
# upgrade of ws, and a check that failed, marked for attention. The oldest
# two are read. It asks no agent anything.
set -euo pipefail

ws="$TRY_WS"

if [ -n "$("$ws" notifications --all 2>/dev/null)" ]; then
	echo "$TRY_DIR already has notifications" >&2
	exit 1
fi

add() { "$ws" notifications add "$@" >/dev/null 2>&1; }

add --source ws --kind ws.upgraded "ws was upgraded to v1.4.0, from v1.3.2: the daemon restarted onto it"
add --source plugins --kind plugin.update --ref plugin:jira "jira v0.9.0 is out: update it from Plugins"
"$ws" notifications read n1 n2 >/dev/null 2>&1

"$ws" WS_CREW=c1 mission new "The export test is flaky on CI" >/dev/null 2>&1
add --source pulls --kind pr.merged --ref "demo#12" "demo#12 merged"
add --source missions --kind mission.ended --ref the-export-test-is-flaky-on-ci "Mission the-export-test-is-flaky-on-ci ended: landed, with all of Z met"
"$ws" WS_ROLE=orchestrator mission new "Docs still describe the old greeting" >/dev/null 2>&1
add --source pulls --kind pr.checks_failed --attention --ref "demo#14" "demo#14's deploy check failed after it merged"

echo "notifications: $("$ws" notifications | wc -l | tr -d ' ') unread of $("$ws" notifications --all | wc -l | tr -d ' '); the web UI's Notifications, or ws notifications --all"
