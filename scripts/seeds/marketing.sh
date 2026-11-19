#!/usr/bin/env bash
# A team's workspace on an ordinary day, for screenshots: Sorrel's repos, and missions in every court.
#
# What the README's and the site's pictures are taken of, so they can be
# taken again. It asks no agent anything, so it's the same every time.
#
#   scripts/try.sh up marketing
#
# It makes the try's workspace over, so it's a try's only seed, or its
# first: in place of demo and records there's Sorrel, a made-up company
# that sends invoices for small studios, with
#
#   api       a Go service (it builds, and its tests pass)
#   web       its dashboard
#   records   its ADRs, proposals and notes, landed straight on main
#
# their history of the last week, teammates' pull requests on the file
# that answers for GitHub, and a PROCESS.md its plans quote.
#
# Everything it's made of is in scripts/seeds/marketing/: the repos as
# they start (repos/), what teammates' pull requests change (prs/), what
# crew commit at each step (work/<mission>/<n>/), what a verifier shows
# (bundle/), what agents answer (agent/), and a prototype (artifacts/).
#
# Every agent is Claude Code's to ws, and no model is asked anything:
# stand-in/claude is first on the try's PATH (scripts/try.sh says how).
# As crew it waits to be closed: the seed does their work as them, and
# reports what their hooks would. Asked for an answer (a conversation's
# turn, the orchestrator's run) it plays a script from agent/, which says
# what it reads, what it runs (for real: its proposals are ws's own), and
# what it says. So a mission moves as ws moves it: when a step's checks
# are met, a review approves, a pull request opens, or the orchestrator's
# script accepts a step.
#
# It needs go (the plans' checks are go test, which ws runs), and curl
# and node, for what only the web UI's pages and "GitHub" can be told.
#
# Everything here happened in the minute it was seeded: ws has no clock
# to set, so the pages say "now" where a real workspace says "2h".
set -euo pipefail

for tool in go curl node; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "marketing needs $tool" >&2
		exit 1
	}
done

ws="$TRY_WS"
dir="$TRY_DIR"
from="$TRY_REPO/scripts/seeds/marketing"

if [ -e "$dir/.ws" ]; then
	echo "marketing makes the try's workspace over: it's a try's only seed, or its first" >&2
	exit 1
fi

# --- git, as test-workspace.sh has it -----------------------------------

# Days ago, as dates: what's happened is dated back from today.
day() { date -u -v-"$1"d +%Y-%m-%d 2>/dev/null || date -u -d "$1 days ago" +%Y-%m-%d; }
d1="$(day 1)" d2="$(day 2)" d3="$(day 3)" d4="$(day 4)" d5="$(day 5)" d6="$(day 6)" d9="$(day 9)"

# git as someone, at a time, unsigned whatever the global config says.
git_as() { # <author> <when> <git args...>
	local author="$1" when="$2"
	shift 2
	GIT_AUTHOR_DATE="$when" GIT_COMMITTER_DATE="$when" git -c user.name="$author" -c user.email="${author%%\[*}@example.com" -c commit.gpgsign=false "$@"
}

# overlay <from> <to>: the files under <from>, over those under <to>.
overlay() { cp -R "$1"/. "$2"/; }

sha() { git -C "$dir/.remotes/$1" rev-parse "$2"; }

# landed <repo> <author> <when> <subject> <what>: a commit on the remote's
# main, of scripts/seeds/marketing/<what> laid over it.
landed() {
	local src="$dir/.remotes/$1"
	overlay "$from/$5" "$src"
	git -C "$src" add -A
	git_as "$2" "$3" -C "$src" commit -q -m "$4"
}

# pr_branch <repo> <number> <branch> <base> <author> <when> <subject> [what]:
# a commit on a pull request's branch in the remote, of prs/<repo>/<what>
# (its number, unless it's said) laid over its base, with GitHub's
# refs/pull/<number>/head pointing at it.
pr_branch() {
	local src="$dir/.remotes/$1"
	git -C "$src" checkout -q -B "$3" "$4"
	overlay "$from/prs/$1/${8:-$2}" "$src"
	git -C "$src" add -A
	git_as "$5" "$6" -C "$src" commit -q -m "$7"
	git -C "$src" update-ref "refs/pull/$2/head" HEAD
	git -C "$src" checkout -q main
}

# --- the workspace, made over --------------------------------------------

rm -rf "$dir/repos/demo" "$dir/repos/records" "$dir/.remotes"
mkdir -p "$dir/.remotes"

for name in api web records; do
	mkdir -p "$dir/.remotes/$name"
	overlay "$from/repos/$name" "$dir/.remotes/$name"
	git -C "$dir/.remotes/$name" init -q --initial-branch=main
done
# A commit you reviewed on "GitHub" stays fetchable after a force-push,
# and records takes pushes to main (ws land pushes there).
git -C "$dir/.remotes/api" config uploadpack.allowAnySHA1InWant true
git -C "$dir/.remotes/web" config uploadpack.allowAnySHA1InWant true
git -C "$dir/.remotes/records" config receive.denyCurrentBranch updateInstead

# The last week on each main: pull requests squashed in on api and web
# (the file that answers for GitHub has them as merged), and commits
# pushed straight to records.
git -C "$dir/.remotes/api" add -A
git_as maya "${d9}T10:00:00Z" -C "$dir/.remotes/api" commit -q -m "Start the api"
landed api maya "${d5}T11:20:00Z" "Send invoice.paid when a payment settles (#36)" merged/api/36
landed api tomas "${d3}T15:40:00Z" "Validate currency codes (#38)" merged/api/38

git -C "$dir/.remotes/web" add -A
git_as ines "${d9}T10:30:00Z" -C "$dir/.remotes/web" commit -q -m "Start the dashboard"
landed web ines "${d4}T09:50:00Z" "Sort the invoice list, newest first (#24)" merged/web/24

# said <author> <when> <subject> <path>...: a commit on records' main, of
# those of its files.
said() {
	local author="$1" when="$2" subject="$3"
	shift 3
	git -C "$dir/.remotes/records" add "$@"
	git_as "$author" "$when" -C "$dir/.remotes/records" commit -q -m "$subject"
}
said maya "${d9}T11:00:00Z" "Start the records" README.md adr/0001-money-is-whole-minor-units.md
said tomas "${d6}T14:10:00Z" "Add ADR 0002: webhooks are delivered at least once" adr/0002-webhooks-are-delivered-at-least-once.md
said ines "${d5}T16:30:00Z" "Add ADR 0003: an invoice freezes when it's sent" adr/0003-an-invoice-freezes-when-it-is-sent.md
said jun "${d4}T10:15:00Z" "Propose export invoices as CSV" proposals/csv-export.md
said ines "${d2}T13:45:00Z" "Add a note: how VAT is shown on an invoice" notes/how-vat-is-shown-on-an-invoice.md
said maya "${d1}T17:05:00Z" "Propose usage-based billing" proposals/usage-based-billing.md

# Teammates' open pull requests: a stack of two, one that's back with
# you (pushed to since you asked for changes), a bot's and a draft.
pr_branch api 41 maya/idempotency-keys main maya "${d2}T10:05:00Z" "Idempotency keys for POST /invoices"
pr_branch api 43 tomas/rate-cache main tomas "${d3}T09:30:00Z" "Cache exchange rates for an hour"
pr_branch api 44 tomas/rate-refresh tomas/rate-cache tomas "${d2}T16:20:00Z" "Refresh exchange rates in the background"
pr_branch api 39 dependabot/go-1.23 main "dependabot[bot]" "${d5}T03:00:00Z" "Bump go from 1.22 to 1.23"
pr_branch web 27 ines/keyboard-shortcuts main ines "${d2}T11:40:00Z" "Keyboard shortcuts for the invoice list"
pr_branch web 25 jun/empty-state main jun "${d4}T14:00:00Z" "An empty state for a new account"
jun_reviewed="$(sha web jun/empty-state)"
pr_branch web 25 jun/empty-state jun/empty-state jun "${d1}T08:10:00Z" "Say what goes here, and offer the first invoice" 25b
pr_branch web 28 maya/onboarding main maya "${d1}T15:00:00Z" "WIP: an onboarding checklist"

cat >"$dir/ws.toml" <<'EOF'
[workspace]
org = "example-org"
default_branch = "main"
display_name = "Sorrel"

[repos.api]
display_name = "API"

[repos.web]
display_name = "Web"
aliases = ["fe"]

[repos.records]
display_name = "Records"
aliases = ["rec"]
landing = "direct"

[missions]
records = "records"
# Guarantees: what ws holds on every mission, whatever an agent reads.
review_before_pr = true

[artifacts]
path = "records:artifacts/{date}-{slug}/"

[documents.adr]
title = "ADR"
path = "records:adr/{number}-{slug}.md"
add = "Add ADR {number}: {slug}"
amend = "Amend ADR {number}: {slug}"
guide = "records:README.md"

[documents.proposal]
title = "Proposal"
path = "records:proposals/{slug}.md"
add = "Propose {title}"
amend = "Amend the proposal {slug}"
guide = "records:README.md"

[documents.note]
title = "Note"
path = "records:notes/{slug}.md"
add = "Add a note: {title}"
amend = "Amend the note {slug}"
EOF

cat >"$dir/PROCESS.md" <<'EOF'
# How we work

Bugs are reproduced first, with a failing test, then fixed. I see the
cause before the fix is written.

Anything that changes what customers see is proposed first, as a proposal
in records. Once it's accepted, it can be built.

A change to the dashboard is shown working in a browser, by someone who
didn't build it.

I review every change before its pull request is opened, and only I
merge. Tend to my pull requests: fix failing checks, and draft replies to
review comments for me to read.

Decisions worth keeping are written up as ADRs in records.
EOF
git -C "$dir" add ws.toml PROCESS.md
git_as maya "${d9}T10:05:00Z" -C "$dir" commit -q -m "Sorrel's workspace"

for name in api web records; do
	repo="$dir/repos/$name"
	mkdir -p "$repo"
	git clone -q --bare "$dir/.remotes/$name" "$repo/.bare"
	git -C "$repo/.bare" config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'
	git -C "$repo/.bare" fetch -q --all
	git -C "$repo/.bare" worktree add -q "$repo/.ground" main
done

# Per machine: Claude Code in every role, which here is the stand-in.
cat >"$dir/ws.local.toml" <<'EOF'
[crew]
harness = "claude"
model = "sonnet"
allowed = ["claude:opus", "claude:haiku"]
trust_folders = true

[briefing]
harness = "claude"
model = "opus"

[orchestrator]
harness = "claude"
model = "haiku"

[conversation]
harness = "claude"
model = "sonnet"

[judge]
harness = "claude"
model = "haiku"

[github]
fixture = ".remotes/github.json"
EOF

url="https://github.com/example-org"
cat >"$dir/.remotes/github.json" <<JSON
{
  "viewer": "you",
  "requested": [{"repo": "web", "number": 27}],
  "repos": {
    "api": {
      "prs": [
        {"number": 41, "title": "Idempotency keys for POST /invoices", "authorLogin": "maya", "headRefName": "maya/idempotency-keys", "baseRefName": "main", "headRefOid": "$(sha api maya/idempotency-keys)",
         "url": "$url/api/pull/41", "statusRollup": "success", "reviewDecision": "REVIEW_REQUIRED", "additions": 68, "deletions": 0, "changedFiles": 2,
         "createdAt": "${d2}T10:05:00Z", "updatedAt": "${d1}T09:20:00Z",
         "requested": [{"login": "you"}],
         "latest": [{"author": "tomas", "state": "APPROVED", "submittedAt": "${d1}T09:20:00Z", "commit": "$(sha api maya/idempotency-keys)"}]},
        {"number": 43, "title": "Cache exchange rates for an hour", "authorLogin": "tomas", "headRefName": "tomas/rate-cache", "baseRefName": "main", "headRefOid": "$(sha api tomas/rate-cache)",
         "url": "$url/api/pull/43", "statusRollup": "success", "reviewDecision": "APPROVED", "additions": 47, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d3}T09:30:00Z", "updatedAt": "${d2}T15:40:00Z",
         "latest": [{"author": "you", "state": "APPROVED", "submittedAt": "${d2}T15:40:00Z", "commit": "$(sha api tomas/rate-cache)"}]},
        {"number": 44, "title": "Refresh exchange rates in the background", "authorLogin": "tomas", "headRefName": "tomas/rate-refresh", "baseRefName": "tomas/rate-cache", "headRefOid": "$(sha api tomas/rate-refresh)",
         "url": "$url/api/pull/44", "statusRollup": "pending", "reviewDecision": "REVIEW_REQUIRED", "additions": 42, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d2}T16:20:00Z", "updatedAt": "${d1}T08:05:00Z",
         "requested": [{"login": "you"}]},
        {"number": 39, "title": "Bump go from 1.22 to 1.23", "authorLogin": "dependabot[bot]", "authorIsBot": true, "headRefName": "dependabot/go-1.23", "baseRefName": "main", "headRefOid": "$(sha api dependabot/go-1.23)",
         "url": "$url/api/pull/39", "statusRollup": "success", "additions": 1, "deletions": 1, "changedFiles": 1,
         "createdAt": "${d5}T03:00:00Z", "updatedAt": "${d5}T03:00:00Z"}
      ],
      "merged": [
        {"number": 36, "title": "Send invoice.paid when a payment settles", "authorLogin": "maya", "headRefName": "maya/invoice-paid", "baseRefName": "main",
         "url": "$url/api/pull/36", "state": "MERGED", "additions": 12, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d6}T13:00:00Z", "updatedAt": "${d5}T11:20:00Z", "mergedAt": "${d5}T11:20:00Z",
         "latest": [{"author": "you", "state": "APPROVED", "submittedAt": "${d5}T10:50:00Z"}]},
        {"number": 38, "title": "Validate currency codes", "authorLogin": "tomas", "headRefName": "tomas/currency-codes", "baseRefName": "main",
         "url": "$url/api/pull/38", "state": "MERGED", "additions": 16, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d4}T10:00:00Z", "updatedAt": "${d3}T15:40:00Z", "mergedAt": "${d3}T15:40:00Z",
         "latest": [{"author": "maya", "state": "CHANGES_REQUESTED", "submittedAt": "${d4}T16:00:00Z"}, {"author": "maya", "state": "APPROVED", "submittedAt": "${d3}T15:10:00Z"}]}
      ],
      "stacks": [
        {"number": 1, "base": "main", "open": true, "prs": [{"number": 43, "state": "open"}, {"number": 44, "state": "open"}]}
      ],
      "bodies": {
        "41": "A client that times out and tries again shouldn't send an invoice twice.\n\n\`Idempotent\` remembers what it answered for each \`Idempotency-Key\`, and answers a second POST with that, marked \`Idempotent-Replayed\`.\n\n- [x] tests\n- [ ] keys are kept in memory: they want a home that outlives a deploy",
        "43": "Listing invoices in another currency asked the bank for a rate once a row. This keeps each rate for an hour.\n\nBottom of a stack: #44 refreshes them in the background.",
        "44": "Asks the bank again for every rate the cache holds, every half hour, so nobody's request waits for it.",
        "39": "Bumps go from 1.22 to 1.23."
      },
      "said": {
        "41": [
          {"id": "R_api_41_1", "author": "tomas", "state": "APPROVED", "body": "Reads well. One for later: the map only ever grows.", "at": "${d1}T09:20:00Z", "url": "$url/api/pull/41#pullrequestreview-1", "commit": "$(sha api maya/idempotency-keys)"}
        ]
      },
      "threads": {
        "41": [
          {"id": "T_api_41_1", "path": "internal/httpapi/idempotency.go", "line": 14, "originalLine": 14, "side": "RIGHT",
           "comments": [
             {"author": "tomas", "body": "question: nothing takes a key out of this map. Is that for this pull request, or the one that gives them a home?", "at": "${d1}T09:18:00Z", "url": "$url/api/pull/41#discussion_r1",
              "commit": "$(sha api maya/idempotency-keys)", "originalCommit": "$(sha api maya/idempotency-keys)"},
             {"author": "maya", "body": "The next one: they'll expire where they're kept.", "at": "${d1}T09:40:00Z"}
           ]}
        ]
      }
    },
    "web": {
      "prs": [
        {"number": 27, "title": "Keyboard shortcuts for the invoice list", "authorLogin": "ines", "headRefName": "ines/keyboard-shortcuts", "baseRefName": "main", "headRefOid": "$(sha web ines/keyboard-shortcuts)",
         "url": "$url/web/pull/27", "statusRollup": "failure", "reviewDecision": "REVIEW_REQUIRED", "additions": 11, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d2}T11:40:00Z", "updatedAt": "${d2}T11:40:00Z",
         "requested": [{"team": "dashboard"}]},
        {"number": 25, "title": "An empty state for a new account", "authorLogin": "jun", "headRefName": "jun/empty-state", "baseRefName": "main", "headRefOid": "$(sha web jun/empty-state)",
         "url": "$url/web/pull/25", "statusRollup": "success", "reviewDecision": "CHANGES_REQUESTED", "additions": 9, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d4}T14:00:00Z", "updatedAt": "${d1}T08:10:00Z",
         "latest": [{"author": "you", "state": "CHANGES_REQUESTED", "submittedAt": "${d3}T10:00:00Z", "commit": "$jun_reviewed"}]},
        {"number": 28, "title": "WIP: an onboarding checklist", "authorLogin": "maya", "isDraft": true, "headRefName": "maya/onboarding", "baseRefName": "main", "headRefOid": "$(sha web maya/onboarding)",
         "url": "$url/web/pull/28", "additions": 13, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d1}T15:00:00Z", "updatedAt": "${d1}T15:00:00Z"}
      ],
      "merged": [
        {"number": 24, "title": "Sort the invoice list, newest first", "authorLogin": "ines", "headRefName": "ines/sort", "baseRefName": "main",
         "url": "$url/web/pull/24", "state": "MERGED", "additions": 6, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d5}T15:00:00Z", "updatedAt": "${d4}T09:50:00Z", "mergedAt": "${d4}T09:50:00Z",
         "latest": [{"author": "jun", "state": "APPROVED", "submittedAt": "${d4}T09:30:00Z"}]}
      ],
      "bodies": {
        "27": "\`j\` and \`k\` move through the list, and \`/\` goes to the filter. CI is red: the test for \`/\` wants a browser.",
        "25": "What a studio sees before it has sent anything. Rewritten as asked: it says what will be here, and offers the first invoice.",
        "28": "Three steps for a new studio. Not ready: the steps aren't wired to anything yet."
      },
      "said": {
        "25": [
          {"id": "R_web_25_1", "author": "you", "state": "CHANGES_REQUESTED", "body": "It should say what goes here, not that there's nothing.", "at": "${d3}T10:00:00Z", "url": "$url/web/pull/25#pullrequestreview-1", "commit": "$jun_reviewed"},
          {"id": "C_web_25_2", "author": "jun", "body": "Pushed: it says what will be listed, and the button writes the first one.", "at": "${d1}T08:10:00Z", "url": "$url/web/pull/25#issuecomment-2"}
        ]
      },
      "threads": {
        "25": [
          {"id": "T_web_25_1", "path": "src/invoices/EmptyState.tsx", "outdated": true, "originalLine": 5, "side": "RIGHT",
           "comments": [
             {"author": "you", "body": "suggestion: \"Click here\" says nothing about what it does. Name the thing: \"Write your first invoice\".", "at": "${d3}T10:00:00Z", "url": "$url/web/pull/25#discussion_r1",
              "commit": "$jun_reviewed", "originalCommit": "$jun_reviewed", "diffHunk": "@@ -0,0 +1,8 @@\n+export function EmptyState() {\n+  return (\n+    <div className=\"empty\">\n+      <h2>No invoices</h2>\n+      <p>Click here to make one.</p>"},
             {"author": "jun", "body": "Done.", "at": "${d1}T08:10:00Z"}
           ]}
        ]
      }
    }
  }
}
JSON

mkdir -p "$dir/.try/bin" "$dir/.try/agent"
cp "$from/stand-in/claude" "$dir/.try/bin/claude"
chmod +x "$dir/.try/bin/claude"
cp "$from"/agent/*.md "$dir/.try/agent/"

# --- missions --------------------------------------------------------------

# as <slug> <crew> <ws args...>: a ws command, as that mission's crew.
as() {
	local slug="$1" id="$2"
	shift 2
	"$ws" WS_MISSION="$slug" WS_CREW="$id" "$@"
}

# crew <slug> <id>: wait until the mission has that crew, which ws starts
# by itself on a step that's crew's.
crew() {
	for _ in $(seq 1 100); do
		"$ws" show -m "$1" -f llm "$2" >/dev/null 2>&1 && return 0
		sleep 0.2
	done
	echo "$1: crew $2 never started" >&2
	return 1
}

# says <slug> <id> <event>: what crew's own hooks would report (prompt:
# it's working; stop: it's finished its turn).
says() { as "$1" "$2" mission hook "$3" --root "$dir" </dev/null; }

# at <slug> <n>: wait until the mission is at step n.
at() {
	for _ in $(seq 1 150); do
		"$ws" mission show "$1" -f llm 2>/dev/null | grep -q "^At: Step $2 of" && return 0
		sleep 0.2
	done
	echo "$1 never reached step $2:" >&2
	"$ws" mission show "$1" >&2 || true
	return 1
}

# work <slug> <id> <repo/capsule> <n> <subject>: crew's commit, of
# work/<slug>/<n> laid over their capsule.
work() {
	local capsule="$dir/repos/$3"
	overlay "$from/work/$1/$4" "$capsule"
	git -C "$capsule" add -A
	git -C "$capsule" -c user.name=you -c user.email=you@example.com -c commit.gpgsign=false commit -q -m "$5"
}

# done_with <slug> <id> <repo/capsule> <summary> <<handoff: crew report done.
done_with() {
	local f="$dir/.ws/missions/$1/files/$2-handoff-draft.md"
	mkdir -p "$(dirname "$f")"
	cat >"$f"
	(cd "$dir/repos/$3" && as "$1" "$2" mission done "$4" --handoff "$f" >/dev/null)
	rm -f "$f"
}

# The web UI's API, for what only its pages do: approving a review. This
# is the try's first ws command, so it's what starts the daemon, with the
# stand-in on its PATH.
link="$("$ws" web --print)"
origin="${link%%/auth\?*}"
token="$(printf '%s' "$link" | sed -n 's/.*[?&]t=\([^&]*\).*/\1/p')"

# approve <slug> <repo/capsule> <summary>: your review of the capsule as
# it is, approving it.
approve() {
	local capsule="$dir/repos/$2" base head
	base="$(git -C "$capsule" merge-base HEAD origin/main)"
	head="$(git -C "$capsule" rev-parse HEAD)"
	curl -fsS -X POST -H "Authorization: Bearer $token" -H "Origin: $origin" -H 'Content-Type: application/json' \
		-d "{\"base\":\"$base\",\"head\":\"$head\",\"verdict\":\"approve\",\"summary\":\"$3\"}" \
		"$origin/api/missions/$1/capsules/$2/review/submit" >/dev/null
}

# comment <slug> <repo/capsule> <path> <line> <kind> <words>: a comment in
# your review of the capsule, a draft until it's submitted.
comment() {
	local capsule="$dir/repos/$2" base head
	base="$(git -C "$capsule" merge-base HEAD origin/main)"
	head="$(git -C "$capsule" rev-parse HEAD)"
	curl -fsS -X POST -H "Authorization: Bearer $token" -H "Origin: $origin" -H 'Content-Type: application/json' \
		-d "{\"base\":\"$base\",\"head\":\"$head\",\"path\":\"$3\",\"side\":\"new\",\"line\":$4,\"kind\":\"$5\",\"body\":\"$6\"}" \
		"$origin/api/missions/$1/capsules/$2/review/comments" >/dev/null
}

# pr_is <repo> <branch> <json>: what "GitHub" says of the pull request
# open from the branch, laid over what it said.
pr_is() {
	PR_REPO="$1" PR_BRANCH="$2" PR_IS="$3" node -e '
		const fs = require("fs"), file = process.argv[1], e = process.env;
		const all = JSON.parse(fs.readFileSync(file, "utf8"));
		const pr = all.repos[e.PR_REPO].prs.find((p) => p.headRefName === e.PR_BRANCH);
		if (!pr) throw new Error(`no pull request is open from ${e.PR_REPO}:${e.PR_BRANCH}`);
		Object.assign(pr, JSON.parse(e.PR_IS));
		fs.writeFileSync(file, JSON.stringify(all, null, 2));
	' "$dir/.remotes/github.json"
}

# handoff <done> <learned> <left>: a handoff's four sections, short, for
# done_with to read.
handoff() { printf '## Done\n- %s\n\n## Learned\n- %s\n\n## Left\n- %s\n\n## Read first\n- Nothing.\n' "$1" "$2" "$3"; }

# -- Up next: one to brief, one ready, one parked.

"$ws" mission new "Archive invoices older than seven years" >/dev/null

"$ws" mission new "Show VAT by rate on the PDF" --brief "## Goal
An invoice's PDF shows a row of VAT for each rate, as the note in records says it should.

## Acceptance
- An invoice with lines at 20% and 5% shows two rows, each with its net and its VAT.
" >/dev/null
"$ws" WS_MISSION=show-vat-by-rate-on-the-pdf lift api pdf-vat-rows >/dev/null
"$ws" mission plan -m show-vat-by-rate-on-the-pdf --set - >/dev/null <<'EOF'
## Z
- The PDF shows a row of VAT for each rate, merged.
## Stops early
- None.
## Steps
1. Work: **The rows.** One for each rate, with its net and its VAT.
   - in: `api/pdf-vat-rows`
   - check: `go test ./...` passes
2. Review: **Read it.** Before its pull request. "I review every change before its pull request is opened"
3. Open Pull Request: **Open it.**
   - in: `api`
EOF

"$ws" mission new "Invoices in more than one currency" --brief "## Goal
A studio can send an invoice in the currency its customer pays in, and see what it's worth in its own.

## Context
- Waits on the exchange rates tomas is caching (api#43, api#44).
" >/dev/null
"$ws" mission park invoices-in-more-than-one-currency >/dev/null

# -- Waiting: one on another team, one whose pull request is in review.

slug="rotate-the-webhook-signing-key"
"$ws" mission new "Rotate the webhook signing key" --brief "## Goal
Webhooks are signed with a new key, and the old one is gone, with no delivery refused in between.

## Context
- The api signs with either key while both are in the vault. The platform team adds the new one.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api key-rotation >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- Webhooks are signed with the new key, and the old one is gone.
## Stops early
- None.
## Steps
1. Review: **Ready to rotate.** The api signs with either key.
2. Wait: **The new key is in the vault.** The platform team adds it, in prd.
   - on: the platform team
3. Work: **Drop the old key.** Sign with the new one alone.
   - in: `api/key-rotation`
   - check: `go test ./...` passes
EOF
"$ws" mission accept -m "$slug" "It signs with either. Asked the platform team for the new key on Monday." >/dev/null

slug="paginate-the-invoices-endpoint"
"$ws" mission new "Paginate the invoices endpoint" --brief "## Goal
\`GET /invoices\` gives a page at a time, so a studio with thousands of invoices gets an answer.

## Acceptance
- \`?limit=\` and \`?after=\` page through every invoice once, in order.
- A request with neither gets the first fifty.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api paginate >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- `GET /invoices` gives a page at a time, merged.
## Stops early
- None.
## Steps
1. Work: **Page it.** `?limit=` invoices after the one numbered `?after=`.
   - in: `api/paginate`
   - check: `go test ./...` passes
2. Review: **Read it.** Before its pull request. "I review every change before its pull request is opened"
3. Open Pull Request: **Open it.**
   - in: `api`
EOF
"$ws" mission dispatch -m "$slug" --capsule api/paginate "Page GET /invoices: ?limit= and ?after=." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
work "$slug" c1 api/paginate 1 "feat(httpapi): GET /invoices gives a page at a time"
handoff '`page` in `internal/httpapi/page.go` takes `?limit=` and `?after=`, and `GET /invoices` answers with `next`, the number to ask after.' 'The cursor is an invoice'"'"'s number: they are unique, and already the order of the list.' 'The dashboard still asks for everything: it gets the first fifty until it follows `next`.' |
	done_with "$slug" c1 api/paginate "GET /invoices gives fifty at a time, with the number to ask after next."
at "$slug" 2
approve "$slug" api/paginate "Good. The dashboard follows next in a mission of its own."
at "$slug" 3
says "$slug" c1 stop
pr_is api paginate '{"statusRollup": "pending", "reviewDecision": "REVIEW_REQUIRED", "additions": 61, "deletions": 3, "changedFiles": 3, "requested": [{"login": "maya"}]}'

# -- In progress: crew at work on two.

slug="dark-mode-for-the-dashboard"
"$ws" mission new "Dark mode for the dashboard" --brief "## Goal
The dashboard follows the system's theme, light or dark.

## Context
- Every colour is a variable in \`web/src/theme.css\` already.

## Acceptance
- With the system dark, nothing on the invoices page is unreadable or still light.
" >/dev/null
"$ws" WS_MISSION="$slug" lift web dark-mode >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- The dashboard follows the system's theme, shown in a browser, merged.
## Stops early
- None.
## Steps
1. Work: **The dark theme.** The variables in `theme.css`, under `prefers-color-scheme: dark`.
   - in: `web/dark-mode`
   - makes: the theme, and a decision saying how the dashboard is run
2. Verify: **Browser check.** The invoices page, light and dark. "A change to the dashboard is shown working in a browser, by someone who didn't build it."
   - in: `web/dark-mode`
   - check: with the system dark, the page's background is dark and its text is light
   - yours: the dark theme looks like Sorrel
3. Review: **Approve the browser check.** The bundle from step 2, and the change. "I review every change before its pull request is opened"
4. Open Pull Request: **Open it.**
   - in: `web`
EOF
"$ws" mission dispatch -m "$slug" --on claude:opus --capsule web/dark-mode "Write the dark theme: the variables in theme.css, under prefers-color-scheme: dark." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt

slug="reminders-for-overdue-invoices"
"$ws" mission new "Reminders for overdue invoices" --brief "## Goal
A customer is reminded of an invoice that's overdue, so a studio doesn't have to chase it by hand.

## Context
- It changes what customers see, so it's proposed first.

## Acceptance
- The proposal is in records, read by me.
- A reminder goes seven days after the due date, and \`invoice.reminded\` with it.
" >/dev/null
"$ws" WS_MISSION="$slug" lift records overdue-reminders >/dev/null
"$ws" WS_MISSION="$slug" lift api overdue-reminders >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- The proposal has landed in records, and the reminder is sent, merged.
## Stops early
- The proposal is turned down: that's the answer.
## Steps
1. Work: **Propose it.** What's sent, when, and who can turn it off. "Anything that changes what customers see is proposed first"
   - in: `records/overdue-reminders`
   - makes: `proposals/overdue-reminders.md`
2. Review: **Read the proposal.** `proposals/overdue-reminders.md`, before it lands.
3. Land: **Land it.** The proposal, on records' main.
   - in: `records/overdue-reminders`
4. Work: **Send the reminder.** The daily job, and `invoice.reminded`.
   - in: `api/overdue-reminders`
   - check: `go test ./...` passes
5. Review: **Read it.** Before its pull request. "I review every change before its pull request is opened"
6. Open Pull Request: **Open it.**
   - in: `api`
EOF
"$ws" mission dispatch -m "$slug" --capsule records/overdue-reminders "Write the proposal: what's sent, when, and who can turn it off." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
work "$slug" c1 records/overdue-reminders 1 "Propose reminders for overdue invoices"
(cd "$dir/repos/records/overdue-reminders" && as "$slug" c1 mission artifact add proposals/overdue-reminders.md "Reminders for overdue invoices" --kind proposal >/dev/null)
handoff 'The proposal, `proposals/overdue-reminders.md`: a reminder at seven days and at twenty-one, the studio copied in, and a switch for each customer.' 'Late fees are left out: they are a decision about money.' 'The build: a daily job in the api, and `invoice.reminded`.' |
	done_with "$slug" c1 records/overdue-reminders "The proposal is written: a reminder at seven days and at twenty-one, with a switch for each customer."
at "$slug" 2 # step 1 has no checks: the orchestrator accepts it (agent/$slug-orchestrator-1.md)
says "$slug" c1 stop
"$ws" mission accept -m "$slug" "Seven and twenty-one days. No late fees in this." >/dev/null
at "$slug" 3
"$ws" land records overdue-reminders >/dev/null
at "$slug" 4
crew "$slug" c2
says "$slug" c2 prompt

# -- Needs you, oldest first: a review, a bundle to verify, a question, a pull request to merge.

# A bug, at the review before its pull request, which you have begun.
slug="invoice-totals-off-by-a-penny"
"$ws" mission new "Invoice totals off by a penny" --brief "## Goal
An invoice's total is what its lines and its VAT add up to, to the penny.

## Context
- Support has three studios whose invoices are a penny over. INV-0388 says £3.57, and its lines and VAT add up to £3.56.
- Amounts are whole minor units (ADR 0001), so this isn't a float.
- A sent invoice is frozen (ADR 0003): a fix applies to invoices sent after it.

## Acceptance
- The cause is named, with a test that fails because of it.
- That test passes, and so does everything else.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api tax-rounding >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- An invoice's total is what its lines and VAT add up to, with a regression test, merged.
## Stops early
- It can't be reproduced: that's an answer for support.
## Steps
1. Work: **Reproduce it.** As a failing test, and find the cause. "Bugs are reproduced first, with a failing test, then fixed."
   - in: `api/tax-rounding`
   - makes: the failing test, and the cause
   - check: `go test ./internal/invoice -run TestTaxRoundsOnce` fails
2. Review: **Is this the cause?** The failing test, and what its crew found. "I see the cause before the fix is written."
3. Work: **Fix it.** So the total is what a customer adds up.
   - carrying on from: 1
   - check: `go test ./...` passes
4. Review: **Read the fix.** Before its pull request. "I review every change before its pull request is opened"
5. Open Pull Request: **Open it.**
   - in: `api`
EOF
"$ws" mission dispatch -m "$slug" --on claude:opus --capsule api/tax-rounding "Reproduce INV-0388 as a failing test in internal/invoice, and find the cause." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
work "$slug" c1 api/tax-rounding 1 "test(invoice): an invoice whose total is a penny over"
done_with "$slug" c1 api/tax-rounding "Reproduced: tax is rounded on each line, then added up. Three lines at 99p come to 60p of VAT, where the VAT on £2.97 is 59p." <<'EOF'
## Done
- `TestTaxRoundsOnce` in `internal/invoice/invoice_test.go` is INV-0388, and fails: `Tax() = 60, want 59`.

## Learned
- The cause is `Invoice.Tax`: it adds up `Line.Tax`, which rounds each line's tax to a penny. The PDF shows one VAT figure for each rate, worked out on the net at that rate, so the two differ whenever the lines' remainders add up to half a penny or more.
- It isn't a float: every amount is `Money`, whole minor units.
- Nothing a customer sees uses a line's own tax.

## Left
- The fix: work the tax out for each rate on what its lines come to together, and round once.

## Read first
- `internal/invoice/invoice.go`
EOF
at "$slug" 2
"$ws" mission accept -m "$slug" "That's the cause. Round once for each rate, as the PDF shows it." >/dev/null
at "$slug" 3
says "$slug" c1 prompt
work "$slug" c1 api/tax-rounding 2 "fix(invoice): tax is rounded once for each rate, not on every line"
done_with "$slug" c1 api/tax-rounding "Fixed: tax is worked out for each rate on what its lines come to, and rounded once. INV-0388 comes to £3.56." <<'EOF'
## Done
- `Invoice.TaxByRate` works the tax out for each rate on the net of its lines, rounded once. `Invoice.Tax` adds those up, so the total is what the PDF's rows add up to.
- `Line.Tax` is gone: nothing a customer sees used it.
- `TestTaxRoundsOnce` passes, and `TestTaxByRate` covers an invoice with two rates.

## Learned
- Invoices already sent keep the total they were sent with (ADR 0003). The three support has are a penny over, and stay so.

## Left
- Whether support writes to those three studios is theirs to say.

## Read first
- `internal/invoice/invoice.go`
EOF
at "$slug" 4
says "$slug" c1 stop
comment "$slug" api/tax-rounding internal/invoice/invoice.go 55 question "Half away from zero, as before? Worth saying which in ADR 0001, now this is the one place tax is rounded."


slug="export-invoices-as-csv"
"$ws" mission new "Export invoices as CSV" --brief "## Goal
A studio downloads the invoices its page is showing, filter included, as a CSV for its accountant.

## Context
- Proposed in records (\`proposals/csv-export.md\`), and accepted.
- One row an invoice. Amounts as a customer reads them, not minor units.

## Acceptance
- **api:** \`GET /invoices.csv\` takes the filter \`GET /invoices\` does.
- **web:** an Export button in the invoices toolbar, which can't be pressed with nothing to export.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api csv-export >/dev/null
"$ws" WS_MISSION="$slug" lift web csv-export >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- The invoices page exports what it shows as a CSV, shown working in a browser, merged in api and web.
## Stops early
- None.
## Steps
1. Work: **The endpoint.** `GET /invoices.csv`, which takes the filter `GET /invoices` does.
   - in: `api/csv-export`
   - check: `go test ./...` passes
2. Work: **The button.** Export, in the invoices toolbar, which downloads what the filter shows.
   - in: `web/csv-export`
   - makes: the button, and a decision saying how the dashboard is run against the api
3. Verify: **Browser check.** The export, running. "A change to the dashboard is shown working in a browser, by someone who didn't build it."
   - in: `web/csv-export`
   - check: the file has the invoices the filter shows, and no others
   - check: Export can't be pressed while there's nothing to export
   - yours: the button sits well in the toolbar, light and dark
4. Review: **Approve the browser check.** The bundle from step 3, and both changes. "I review every change before its pull request is opened"
5. Open Pull Request: **Open the api's.**
   - in: `api`
6. Open Pull Request: **Open the dashboard's.**
   - in: `web`
EOF
"$ws" mission dispatch -m "$slug" --capsule api/csv-export "Write GET /invoices.csv, which takes the filter GET /invoices does." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
work "$slug" c1 api/csv-export api "feat(httpapi): GET /invoices.csv, a row an invoice"
handoff '`ExportInvoices` in `internal/httpapi/export.go`, at `GET /invoices.csv`: a row an invoice, kept to `?customer=`.' 'Amounts are written as a customer reads them (`1094.40`), as the proposal asks.' 'The button in the dashboard.' |
	done_with "$slug" c1 api/csv-export "GET /invoices.csv gives a row an invoice, kept to the filter."
at "$slug" 2
says "$slug" c1 stop
crew "$slug" c2
says "$slug" c2 prompt
work "$slug" c2 web/csv-export web "feat(invoices): export what the list shows as a CSV"
(cd "$dir/repos/web/csv-export" && as "$slug" c2 mission decide "The dashboard is run with pnpm dev, against the api on :8080 (go run ./cmd/api, in its capsule)" --why "The verifier needs both running to see the export work" >/dev/null 2>&1) || true
handoff '`ExportButton` in `src/invoices/ExportButton.tsx`, at the end of the toolbar: it downloads `/invoices.csv` with the filter as it stands.' 'With nothing to export it is a disabled button, not a link, so it takes no click and no focus.' 'Nothing.' |
	done_with "$slug" c2 web/csv-export "Export is in the invoices toolbar, and downloads what the filter shows."
at "$slug" 3 # step 2 has no checks: the orchestrator accepts it (agent/$slug-orchestrator-1.md)
says "$slug" c2 stop
crew "$slug" c3
says "$slug" c3 prompt
mkdir -p "$dir/.ws/missions/$slug/files"
cp "$from"/bundle/* "$dir/.ws/missions/$slug/files/"
(cd "$dir/repos/web/csv-export" && as "$slug" c3 mission bundle --set - >/dev/null <<'EOF'
## Summary
Both checks are met. The file holds the three invoices the filter shows, and Export can't be pressed when the filter shows none. How the button sits in the toolbar is yours to say.

## Try it
- try: `pnpm install && pnpm dev`
- look for: Invoices, on http://localhost:5173
- look for: Export, at the end of the toolbar

## Checks
1. met: With the filter at "marlow", the page showed three invoices, and the file had those three and no others.
   - point: The page: INV-0042, INV-0039 and INV-0031.
   - point: The file: a heading and the same three rows.
   - shows: `mission:download.txt` The file, as the api gave it
   - shows: `mission:invoices-light.svg` The page, filtered to marlow
2. met: With a filter that matches nothing, Export is grey, and a click on it does nothing.
   - shows: `mission:invoices-empty.svg` No invoices, and Export greyed
3. yours: Export is the last thing in the toolbar, after the filter and as tall as it.
   - shows: `mission:invoices-light.svg` Light
     - before: `mission:invoices-light-before.svg`
   - shows: `mission:invoices-dark.svg` Dark
     - before: `mission:invoices-dark-before.svg`

## Not verified
- Safari: it isn't installed here.
- A list of more than fifty invoices: the sample has 41.

## Noticed
- An amount in yen is written with two decimals (`1200.00`), and yen has none.
EOF
)
at "$slug" 4
says "$slug" c3 stop

slug="retry-failed-webhooks-with-backoff"
"$ws" mission new "Retry failed webhooks with backoff" --brief "## Goal
A webhook that isn't delivered is tried again, further apart each time, until it is or we give up and say so.

## Context
- ADR 0002: delivered at least once. It doesn't say how long we keep trying.

## Acceptance
- A delivery that fails is tried again, with the same \`Sorrel-Delivery\`.
- One that's given up on is marked failed.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api webhook-retries >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- A failed delivery is retried with backoff, and marked failed when it's given up on, merged.
## Stops early
- None.
## Steps
1. Work: **Retry with backoff.** Further apart each time, with the same `Sorrel-Delivery`.
   - in: `api/webhook-retries`
   - check: `go test ./...` passes
2. Review: **Read it.** Before its pull request. "I review every change before its pull request is opened"
3. Open Pull Request: **Open it.**
   - in: `api`
EOF
"$ws" mission dispatch -m "$slug" --capsule api/webhook-retries "Retry a failed delivery with backoff, and mark it failed when it's given up on." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
as "$slug" c1 mission ask "How long do we keep trying before a delivery is marked failed? ADR 0002 leaves it open. Three days, as Stripe does, covers a weekend's outage; one day keeps a customer's books from being wrong for long." >/dev/null
says "$slug" c1 stop

slug="rate-limit-the-public-api"
"$ws" mission new "Rate-limit the public API" --brief "## Goal
One customer's script can't slow the api for everyone else.

## Acceptance
- A key that makes more than its limit of requests in a minute is answered 429, with \`Retry-After\`.
" >/dev/null
"$ws" WS_MISSION="$slug" lift api rate-limit >/dev/null
"$ws" mission plan -m "$slug" --set - >/dev/null <<'EOF'
## Z
- A key over its limit is answered 429 with Retry-After, merged.
## Stops early
- None.
## Steps
1. Work: **Limit each key.** So many requests a minute, then 429 with `Retry-After`.
   - in: `api/rate-limit`
   - check: `go test ./...` passes
2. Review: **Read it.** Before its pull request. "I review every change before its pull request is opened"
3. Open Pull Request: **Open it.**
   - in: `api`
EOF
"$ws" mission dispatch -m "$slug" --capsule api/rate-limit "Limit each API key to so many requests a minute." >/dev/null
crew "$slug" c1
says "$slug" c1 prompt
work "$slug" c1 api/rate-limit 1 "feat(httpapi): each API key has so many requests a minute"
handoff '`RateLimit` in `internal/httpapi/ratelimit.go`: a fixed window of a minute for each key, then 429 with `Retry-After`.' 'Counts are in memory, one api at a time: two behind a load balancer each allow the limit.' 'Wiring it into `cmd/api`, with the limit from config.' |
	done_with "$slug" c1 api/rate-limit "Each key has so many requests a minute, then 429 with Retry-After."
at "$slug" 2
approve "$slug" api/rate-limit "Good. In memory is fine while there's one api."
at "$slug" 3
says "$slug" c1 stop
pr_is api rate-limit "{\"statusRollup\": \"success\", \"reviewDecision\": \"APPROVED\", \"additions\": 58, \"deletions\": 0, \"changedFiles\": 2, \"latest\": [{\"author\": \"tomas\", \"state\": \"APPROVED\", \"submittedAt\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}]}"

# -- Conversations, which the stand-in answers from agent/w<n>-1.md.

# talk <id> <what you say> <words its answer has>: a conversation, and its
# agent's answer.
talk() {
	"$ws" talk "$2" --print >/dev/null
	for _ in $(seq 1 150); do
		"$ws" show "$1" 2>/dev/null | grep -q "$3" && return 0
		sleep 0.2
	done
	echo "$1 was never answered:" >&2
	"$ws" show "$1" >&2 || true
	return 1
}
talk w1 "Two studios asked this week for late fees on overdue invoices. Should we do it, and what would it take?" "flat fee or interest"
talk w2 "Where do we round money, and is it the same everywhere?" "half away from zero"

# -- An artifact: two takes on where Export goes, by you.

"$ws" artifact add "$from/artifacts/export-v1.html" --title "Where Export goes" \
	--note "Export at the end of the toolbar, after the filter: it's the last thing you do with a list." >/dev/null
"$ws" artifact add "$from/artifacts/export-v2.html" --to a1 \
	--note "Export under the list, beside the count it exports, so the button says how many." >/dev/null

cat <<EOF
marketing: Sorrel's workspace, with eleven missions.

  /                                            the missions, by whose court each is in
  /missions/invoice-totals-off-by-a-penny      a bug, at the review before its pull request
  /missions/invoice-totals-off-by-a-penny/review/api/tax-rounding   that review
  /missions/export-invoices-as-csv?tab=verify&b=b1   a verifier's bundle, yours to approve
  /reviews                                     teammates' pull requests, and yours
  /documents                                   records: ADRs, proposals and notes
  /conversations/w1                            a conversation, whose agent proposed a mission
  /artifacts/a1                                a prototype, in two versions

Needs you: a review, a bundle to verify, a question from crew, a pull
request to merge. In progress: crew at work on two. Waiting: a pull
request in review, and a step on another team. Up next: one to brief,
one ready, one parked.

Agents are a stand-in for Claude Code (.try/bin/claude). Crew wait, and
do nothing; a conversation and the orchestrator answer only what
.try/agent/ has a script for.
EOF
