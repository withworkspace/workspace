#!/usr/bin/env bash
# Pull requests of yours to merge from ws: one ready, one waiting on its checks, and a stack ready to merge whole.
#
# For looking at merging from the Reviews page, a pull request's review
# and Find:
#
#   scripts/try.sh up merging
#
# demo's stand-in GitHub already has #16 of yours, approved and green:
# ready to merge. This adds #24, yours, approved with its checks still
# running (Merge when ready), and GitHub's stack 2 of yours, #25 under #26,
# both approved and green (Merge stack of 2). Merging on the stand-in
# GitHub is at once: the pull request moves to merged, a stack's layers
# with it. It asks no agent anything.
set -euo pipefail

dir="$TRY_DIR"
src="$dir/.remotes/demo"

# pr <number> <branch> <from> <subject> <file=text>: a pull request's
# commit on demo's remote, by you, its ref where GitHub keeps it.
pr() {
	local number="$1" branch="$2" from="$3" subject="$4" pair="$5"
	git -C "$src" checkout -q -B "$branch" "$from"
	printf '%b' "${pair#*=}" >"$src/${pair%%=*}"
	git -C "$src" add -A
	git -c user.name="ws test" -c user.email=you@example.com -c commit.gpgsign=false -C "$src" commit -q -m "$subject"
	git -C "$src" update-ref "refs/pull/$number/head" HEAD
}

pr 24 you/goodbye main "Say goodbye on the way out" 'goodbye.go=package main\n\nfunc goodbye() string { return "goodbye" }\n'
pr 25 you/flags main "Parse flags with the flag package" 'flags.go=package main\n\nimport "flag"\n\nvar name = flag.String("name", "world", "who to greet")\n'
pr 26 you/flags-help you/flags "Say what each flag does with -h" 'help.go=package main\n\nconst usage = "demo [-name who]"\n'
git -C "$src" checkout -q main

node -e '
	const fs = require("fs"), file = process.argv[1], sha = JSON.parse(process.argv[2]);
	const all = JSON.parse(fs.readFileSync(file, "utf8"));
	const demo = all.repos.demo;
	const url = demo.prs[0].url.replace(/\d+$/, "");
	const now = new Date().toISOString();
	const mine = (number, title, branch, base, more) => ({number, title, authorLogin: "you", headRefName: branch, baseRefName: base,
		headRefOid: sha[branch], url: url + number, reviewDecision: "APPROVED", mergeable: "MERGEABLE", mergeStateStatus: "CLEAN",
		additions: 3, deletions: 0, changedFiles: 1, createdAt: now, updatedAt: now,
		latest: [{author: "alice", state: "APPROVED", submittedAt: now, commit: sha[branch]}], ...more});
	demo.prs.push(
		mine(24, "Say goodbye on the way out", "you/goodbye", "main", {statusRollup: "pending"}),
		mine(25, "Parse flags with the flag package", "you/flags", "main", {statusRollup: "success"}),
		mine(26, "Say what each flag does with -h", "you/flags-help", "you/flags", {statusRollup: "success"}),
	);
	demo.stacks.push({number: 2, base: "main", open: true, prs: [{number: 25, state: "open", headRef: "you/flags"}, {number: 26, state: "open", headRef: "you/flags-help"}]});
	fs.writeFileSync(file, JSON.stringify(all, null, 2));
' "$dir/.remotes/github.json" "$(printf '{"you/goodbye":"%s","you/flags":"%s","you/flags-help":"%s"}' \
	"$(git -C "$src" rev-parse you/goodbye)" "$(git -C "$src" rev-parse you/flags)" "$(git -C "$src" rev-parse you/flags-help)")"

echo "Reviews: demo#16 ready to merge, demo#24 waiting on its checks (Merge when ready), stack 2 (#25, #26) ready to merge whole."
