#!/usr/bin/env bash
# A replay of crew writing a verification bundle: how long would the
# bundle they handed over be, written as this build says to write one?
# It takes crew's own session up to the moment before they handed the
# bundle over, with everything they'd seen and done, and asks them for the
# bundle again, with this build's `ws mission bundle --format` in place of
# the one they read. So what's measured is the guidance, on real work,
# not the model on a made-up case.
#
#   scripts/bundle-replay.sh <transcript.jsonl> [runs] [model]
#
# The transcript is crew's Claude Code session, one that ran
# `ws mission bundle --set`: ~/.claude/projects/<its capsule's path, with
# - for each / and .>/<session>.jsonl, the session being the crew's
# conversation in the mission's record. FORMAT=<file> replays with that
# file's words instead (an older build's, to compare; a draft's, to try).
#
# It prints the bundle as it was handed over and each replay as a row: its
# words in all, in its summary, in its checks, in its longest met check
# and its longest other, and how many files it shows. The replays are kept
# in a folder it names, to read: fewer words isn't better if what
# mattered has gone.
#
# Nothing is run or handed over: the replay has no tools, and its session
# isn't kept. Each run is a model call over the whole session, which can
# be hundreds of thousands of tokens: run it by hand, when what crew are
# told of bundles changes. It needs claude and jq, and `just build` first.
set -euo pipefail

# Run from an agent's session, the replay mustn't take that session's
# mission, crew or ids as its own (as scripts/try.sh drops them).
unset WS_MISSION WS_CREW WS_ROLE WS_RUN WS_CONVERSATION WS_ORIGIN_PWD
unset CLAUDECODE CLAUDE_PID CLAUDE_EFFORT CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH
unset CLAUDE_CODE_SESSION_ID CLAUDE_CODE_CHILD_SESSION CLAUDE_CODE_SESSION_ATTENDED
unset CLAUDE_CODE_MESSAGING_SOCKET CLAUDE_CODE_MESSAGING_TOKEN

if [ $# -lt 1 ]; then
	echo "usage: $0 <transcript.jsonl> [runs] [model]" >&2
	exit 2
fi
transcript="$1"
runs="${2:-1}"
repo="$(cd "$(dirname "$0")/.." && pwd)"

if [ -n "${FORMAT:-}" ]; then
	format="$(cat "$FORMAT")"
else
	[ -x "$repo/bin/workspace" ] || { echo "no bin/workspace: just build first" >&2; exit 1; }
	format="$("$repo/bin/workspace" mission bundle --format)"
fi

# The assistant message that handed the bundle over, and the session cut
# before its first line.
handed="$(jq -rR 'fromjson? | select(.type == "assistant") | .message
	| select([.content[]? | select(.type == "tool_use") | (.input.command? // "") | contains("mission bundle --set")] | any)
	| .id' "$transcript" | tail -1)"
if [ -z "$handed" ]; then
	echo "$transcript: no ws mission bundle --set in it" >&2
	exit 1
fi
at="$(grep -n -m1 "\"id\":\"$handed\"" "$transcript" | cut -d: -f1)"
session="$(jq -rR 'fromjson? | .sessionId? // empty' "$transcript" | awk 'NR == 1')"
model="${3:-$(jq -rR 'fromjson? | select(.type == "assistant") | .message.model? // empty' "$transcript" | tail -1)}"

tmp="${TMPDIR:-/tmp}"
out="$(mktemp -d "${tmp%/}/bundle-replay.XXXXXX")"
work="$(mktemp -d "${tmp%/}/bundle-replay-cwd.XXXXXX")"
# Claude Code keeps a folder's sessions under its path, each / and . a -.
sessions="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects/$(cd "$work" && pwd -P | sed 's/[^A-Za-z0-9]/-/g')"
trap 'rm -rf "$work" "$sessions"' EXIT
mkdir -p "$sessions"
head -n "$((at - 1))" "$transcript" >"$sessions/$session.jsonl"

jq -rR --arg id "$handed" 'fromjson? | select(.type == "assistant" and .message.id == $id) | .message.content[]?
	| select(.type == "tool_use") | .input.command? // empty' "$transcript" |
	awk '/^[ \t]*## Summary/ { on = 1 } /^[ \t]*EOF[ \t]*$/ { on = 0 } on' >"$out/handed-over.md"

prompt="A note from ws, not the user. This is a replay of your session, to measure how bundles are written: tools are off, and nothing you write here is handed over.

You've verified what you're going to verify. Write the bundle you'd hand over now, as your whole reply: exactly the markdown you'd give \`ws mission bundle --set -\`, starting at \`## Summary\`, with no code fence and nothing before or after it.

\`ws mission bundle --format\` has changed since you read it. It now says:

$format"

# lengths prints a bundle's row: where its words are.
lengths() {
	awk -v name="$1" '
	function endcheck() {
		if (!incheck) return
		checks += words
		if (verdict == "met") { if (words > met) met = words } else if (words > other) other = words
		incheck = 0
	}
	{ total += NF }
	/^##[ \t]/ { endcheck(); part = tolower($0); sub(/^##[ \t]+/, "", part); sub(/[ \t]+$/, "", part); next }
	part == "summary" { summary += NF; next }
	part == "checks" && /^[0-9]+[.)][ \t]/ {
		endcheck(); incheck = 1; field = 0
		line = $0; sub(/^[0-9]+[.)][ \t]+/, "", line)
		verdict = "other"
		if (line ~ /^\**met/) verdict = "met"
		sub(/^\**(met|unmet|not met|can.?t tell|cannot tell|yours)\**[:.]?[ \t]*/, "", line)
		words = split(line, w, /[ \t]+/); if (line == "") words = 0
		next
	}
	part == "checks" && incheck && /^[ \t]+[-*][ \t]+point[ \t]*:/ { words += NF - 2; field = 0; next }
	part == "checks" && incheck && /^[ \t]+[-*][ \t]+(shows|lines|try|ask|before|made by)[ \t]*:/ {
		field = 1; if ($0 ~ /^[ \t]+[-*][ \t]+shows/) shows++
		next
	}
	part == "checks" && incheck && !field { words += NF }
	END { endcheck(); printf "%-14s %6d %8d %7d %12d %14d %6d\n", name, total, summary, checks, met, other, shows }
	' "$2"
}

printf "%-14s %6s %8s %7s %12s %14s %6s\n" "" words summary checks "longest met" "longest other" shows
lengths "handed over" "$out/handed-over.md"
for n in $(seq 1 "$runs"); do
	(
		cd "$work"
		claude -p --resume "$session" --fork-session --no-session-persistence \
			--model "$model" --tools "" --strict-mcp-config --output-format json -- "$prompt" </dev/null >"$out/replay-$n.json"
	)
	jq -r '.result // empty' "$out/replay-$n.json" >"$out/replay-$n.md"
	lengths "replay $n" "$out/replay-$n.md"
done
echo
echo "to read: $out"
echo "what a reader takes from the top of each: scripts/bundle-skim.sh $out/*.md"
