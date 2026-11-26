#!/usr/bin/env bash
# What ws heard of: teammates' commits landing (a document and a sketch), and pull requests moving.
#
# For looking at the events ws keeps of what teammates did (ws events
# --heard), which automations will wait on:
#
#   scripts/try.sh up heard
#
# It hears the grounds and the Reviews list once, so where they are is
# noted (ws events hear: what the daemon does by itself, now). Then
# Brendan lands a decision and a sketch on records, and Ana changes the
# sketch (commits.landed, doc.changed, thing.added, thing.changed, each by
# them); on demo's stand-in GitHub, alice's pull request goes red, dave's is
# approved, and Brendan opens one (pr.checks_failed, pr.approved,
# pr.opened). It hears again, which prints what it heard. Run ws events
# --heard there to see them, and ws events -f json for all they say. It
# asks no agent anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"

if [ -d "$dir/repos/records/sketches/queues" ]; then
	echo "$TRY_DIR has records' queues sketch already" >&2
	exit 1
fi

# Where everything is now isn't news.
"$ws" events hear >/dev/null 2>&1 || true

# land_as <name> <email> <capsule> <subject> <path=text>...: a teammate's
# commit, landed on records.
land_as() {
	local name="$1" email="$2" capsule="$3" subject="$4"
	shift 4
	"$ws" lift records "$capsule" >/dev/null 2>&1
	local c="$dir/repos/records/$capsule"
	for pair in "$@"; do
		mkdir -p "$(dirname "$c/${pair%%=*}")"
		printf '%b' "${pair#*=}" >"$c/${pair%%=*}"
	done
	git -C "$c" add -A
	GIT_AUTHOR_NAME="$name" GIT_AUTHOR_EMAIL="$email" GIT_COMMITTER_NAME="$name" GIT_COMMITTER_EMAIL="$email" \
		git -C "$c" -c commit.gpgsign=false commit -q -m "$subject"
	(cd "$c" && "$ws" land >/dev/null 2>&1)
}

land_as "Brendan Ash" brendan@example.com queue-per-tenant "Decide a queue per tenant, and draw it" \
	'decisions/queue-per-tenant.md=# A queue per tenant\n\nOne queue each, so a slow tenant holds up nobody else.\n' \
	'sketches/queues/thing.toml=kind = "sketch"\ntitle = "Queues"\nmain = "queues.excalidraw"\n' \
	'sketches/queues/queues.excalidraw={"elements":[]}\n'
land_as "Ana Ortiz" ana@example.com queues-retry "Show where a retry goes" \
	'sketches/queues/queues.excalidraw={"elements":[{"type":"arrow"}]}\n'

# demo's pull requests move on "GitHub".
node -e '
	const fs = require("fs"), file = process.argv[1];
	const all = JSON.parse(fs.readFileSync(file, "utf8"));
	const prs = all.repos.demo.prs;
	prs.find((p) => p.number === 13).statusRollup = "failure";
	prs.find((p) => p.number === 19).reviewDecision = "APPROVED";
	prs.push({number: 23, title: "Greet by the time of day", authorLogin: "brendan", headRefName: "brendan/time-of-day",
		baseRefName: "main", headRefOid: "2".repeat(40), url: prs[0].url.replace(/\d+$/, "23"), statusRollup: "pending",
		reviewDecision: "REVIEW_REQUIRED", additions: 9, deletions: 1, changedFiles: 1,
		createdAt: new Date().toISOString(), updatedAt: new Date().toISOString()});
	fs.writeFileSync(file, JSON.stringify(all, null, 2));
' "$dir/.remotes/github.json"

"$ws" events hear >/dev/null
"$ws" events --heard
