#!/usr/bin/env bash
# A cold-start test of ws's information graph: a fresh agent, given only a
# ref and a question, answers from what ws can tell it (ws show and what it
# links to) and the files it names. If it can't, a link or a view is
# missing.
#
#   scripts/coldstart.sh <workspace> <mission> <ref|-> "<question>" [model]
#
# A ref of - starts from the mission itself. It runs Claude Code headless,
# limited to reading: ws show, ws review show, git's read commands (not
# git -C, which would let options run commands), and reading files. It
# prints the agent's answer, then each command it ran. Each run costs a
# model call: run them by hand, when the graph changes.
set -euo pipefail

if [ $# -lt 4 ]; then
	echo "usage: $0 <workspace> <mission> <ref|-> \"<question>\" [model]" >&2
	exit 2
fi
root="$(cd "$1" && pwd)"
mission="$2"
ref="$3"
question="$4"
model="${5:-haiku}"

start="ws show -f llm"
if [ "$ref" != "-" ]; then
	start="ws show -f llm $ref"
fi

prompt="You're answering a question about work in a multi-repo workspace managed by the ws tool, for the mission \"$mission\" (WS_MISSION is set). You know nothing about it yet: find out from ws's records. Start with \`$start\`, which shows what you're asked about and what it links to; \`ws show -f llm <ref>\` shows any linked thing in turn. Read files it names when you need to. Don't change anything.

The question: $question

Answer in a few sentences, citing the refs you got it from. If ws's records don't tell you, say exactly what you couldn't find, rather than guessing."

out="$(mktemp)"
trap 'rm -f "$out"' EXIT
(
	cd "$root"
	WS_MISSION="$mission" claude -p --output-format stream-json --verbose \
		--permission-mode dontAsk --no-session-persistence --model "$model" \
		--allowedTools "Read" "Grep" "Glob" \
		"Bash(ws show)" "Bash(ws show *)" "Bash(ws review show *)" \
		"Bash(git log *)" "Bash(git show *)" "Bash(git diff *)" \
		-- "$prompt" >"$out"
)

echo "## Answer"
echo
jq -r 'select(.type == "result") | .result' "$out"
echo
echo "## What it ran"
echo
jq -r 'select(.type == "assistant") | .message.content[]? | select(.type == "tool_use") |
	"- " + (if .name == "Bash" then .input.command elif .name == "Read" then "Read " + .input.file_path else .name + " " + (.input | tostring) end)' "$out"
