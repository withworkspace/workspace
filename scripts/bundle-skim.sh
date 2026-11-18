#!/usr/bin/env bash
# A skim test of a verification bundle: does its top carry the decision?
# A bundle is meant to put the most important thing first, so a reader who
# only skims it should still know what's wrong, how much, and what's left
# to them. This gives a fresh reader, who knows nothing of the work, only
# the bundle's summary and the first sentence of each check, asks those
# questions, and prints what the reader was given and what they answered.
# An answer of "not said" is a bundle whose top doesn't say it.
#
#   scripts/bundle-skim.sh <bundle.md>... [-- model]
#
# A bundle is the markdown given to `ws mission bundle --set -`:
# scripts/bundle-replay.sh keeps what it replays as such files. The
# reader is Claude Code headless with no tools, on haiku unless a model is
# named after --. Each bundle is one small model call. It's for reading,
# not for a score: whether the answers are right is yours to say.
set -euo pipefail

# Run from an agent's session, the reader mustn't take that session's
# mission, crew or ids as its own (as scripts/try.sh drops them).
unset WS_MISSION WS_CREW WS_ROLE WS_RUN WS_CONVERSATION WS_ORIGIN_PWD
unset CLAUDECODE CLAUDE_PID CLAUDE_EFFORT CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH
unset CLAUDE_CODE_SESSION_ID CLAUDE_CODE_CHILD_SESSION CLAUDE_CODE_SESSION_ATTENDED
unset CLAUDE_CODE_MESSAGING_SOCKET CLAUDE_CODE_MESSAGING_TOKEN

model="haiku"
bundles=()
while [ $# -gt 0 ]; do
	if [ "$1" = "--" ]; then
		model="${2:-haiku}"
		break
	fi
	bundles+=("$1")
	shift
done
if [ ${#bundles[@]} -eq 0 ]; then
	echo "usage: $0 <bundle.md>... [-- model]" >&2
	exit 2
fi

# skim prints a bundle's summary and the first sentence of each check.
skim() {
	awk '
	function first(text,    at) {
		at = match(text, /[.!?]["”)]?[ \t]/)
		return at ? substr(text, 1, at) : text
	}
	function endcheck() {
		if (incheck) print head " " first(said)
		incheck = 0
	}
	/^##[ \t]/ { endcheck(); part = tolower($0); sub(/^##[ \t]+/, "", part); sub(/[ \t]+$/, "", part); next }
	part == "summary" && NF { print; next }
	part == "checks" && /^[0-9]+[.)][ \t]/ {
		endcheck(); incheck = 1; field = 0
		if (!checks++) print ""
		said = $0; sub(/^[0-9]+[.)][ \t]+/, "", said)
		head = "Check " $1
		if (match(said, /^\**(met|unmet|not met|can.?t tell|cannot tell|yours)\**[:.]?[ \t]*/)) {
			verdict = substr(said, 1, RLENGTH); gsub(/[*:. \t]/, "", verdict)
			if (verdict ~ /^can/) verdict = "can not tell"
			head = head " " verdict ":"
			said = substr(said, RLENGTH + 1)
		}
		next
	}
	part == "checks" && incheck && /^[ \t]+[-*][ \t]+[A-Za-z ]+:/ { field = 1; next }
	part == "checks" && incheck && !field { line = $0; sub(/^[ \t]+/, "", line); said = said " " line }
	END { endcheck() }
	' "$1"
}

for bundle in "${bundles[@]}"; do
	given="$(skim "$bundle")"
	prompt="You are deciding whether to approve a piece of work. Someone checked it and wrote a report. You have time only to skim the report: below is its summary and the first sentence of each of its checks, and nothing else. You know nothing else about the work.

Answer from this alone, in one short sentence each. Where it doesn't tell you, answer \"not said\": don't guess.

1. What is wrong, if anything?
2. How much is affected? Give the figures.
3. Does the work pass as it is, or does it go back?
4. What is left for you to judge yourself?

The report, skimmed:

$given"
	echo "## $bundle"
	echo
	echo "Given ($(printf '%s' "$given" | wc -w | tr -d ' ') words):"
	echo
	printf '%s\n' "$given" | sed 's/^/    /'
	echo
	echo "Answered:"
	echo
	(
		cd "${TMPDIR:-/tmp}"
		claude -p --no-session-persistence --model "$model" \
			--tools "" --strict-mcp-config -- "$prompt" </dev/null
	) | sed 's/^/    /'
	echo
done
