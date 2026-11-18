#!/usr/bin/env bash
# A mission across capsules in every state its Work tab shows: landed, yours to check, not started.
#
# For looking at the Work tab: capsules in the order they need you, the
# one that's open with its crew, steps, commits and changed files
# (some not committed), and outputs to read.
#
#   scripts/try.sh up work
#
# It adds the mission work-across-repos, at step 4 of 6 (a Review, yours),
# with these capsules (the first two named for the mission):
#
#   records/…      landed straight on main: a note, c1's
#   demo/…         two commits and two files not committed, c2's, who's still there
#   demo/later     lifted for step 6, and untouched
#
# and a handoff each from c1 and c2, the note, and a note of the mission's
# own. Crew here are shells (no agent): the seed does their work as them.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="work-across-repos"

if [ -d ".ws/missions/$slug" ]; then
	echo "$dir already has the mission $slug" >&2
	exit 1
fi

# Crew are shells here, so the steps ws starts by itself cost nothing.
sed -i.bak '/^\[crew\]$/,/^$/ s/^harness = .*/harness = "shell"/; /^\[crew\]$/,/^$/ s/^model = .*//' ws.local.toml && rm -f ws.local.toml.bak
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi

"$ws" mission new "Work across repos" --brief "## Goal
demo greets whoever it's told to, and the records say why.

## Context
- Seeded by the workspace repo's scripts/seeds/work.sh, for looking at the Work tab. Crew are shells: the seed does their work.
" >/dev/null

"$ws" WS_MISSION="$slug" lift records "$slug" >/dev/null
"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
"$ws" WS_MISSION="$slug" lift demo later >/dev/null

"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- \`go run . Ada\` prints hello, Ada, its pull request is open, and the records say why.
## Stops early
- None.
## Steps
1. Work: **Write the note.** Why demo greets by name, as a note in the records.
   - in: \`records/$slug\`
2. Review: **Read the note.** Before it lands.
3. Work: **Greet by name.** greeting takes a name, and falls back to world.
   - in: \`demo/$slug\`
4. Review: **Read the change.** Before its pull request.
5. Open Pull Request: **Open it.** From \`demo/$slug\`.
6. Work: **Say so in the README.** How to greet someone.
   - in: \`demo/later\`
EOF

# crew waits until the mission has crew <id>, which ws starts by itself
# on a step that's crew's.
crew() {
	for _ in $(seq 1 100); do
		"$ws" show -m "$slug" -f llm "$1" >/dev/null 2>&1 && return 0
		sleep 0.2
	done
	echo "crew $1 never started" >&2
	return 1
}
# handoff writes a handoff's four sections, with what was done.
handoff() {
	local f="$dir/.ws/missions/$slug/files/seed-$1.md"
	mkdir -p "$(dirname "$f")"
	printf '## Done\n- %s\n## Learned\n- %s\n## Left\n- Nothing.\n## Read first\n- Nothing.\n' "$2" "$3" >"$f"
	echo "$f"
}
commit() {
	git -C "$1" add -A && git -C "$1" -c user.name=Seed -c user.email=seed@example.com commit -q -m "$2"
}

# Step 1: c1 writes the note in records.
"$ws" mission dispatch -m "$slug" --harness shell --capsule "records/$slug" "You're a shell: the seed writes the note as you." >/dev/null
crew c1
records="$dir/repos/records/$slug"
mkdir -p "$records/notes"
printf '# Why demo greets by name\n\nA greeting that names who it is for is read. One that says world is skipped.\n\n## What changes\n\n- `go run . Ada` prints `hello, Ada`.\n- With no name it still prints `hello, world`.\n' >"$records/notes/greet-by-name.md"
commit "$records" "Add note: why demo greets by name"
(cd "$records" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission artifact add notes/greet-by-name.md "Why demo greets by name" --kind note >/dev/null)
(cd "$records" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "The note is written: why demo greets by name, and what changes" --handoff "$(handoff c1 'The note, `notes/greet-by-name.md`, in one commit.' 'The records land straight on main: there is no pull request to open.')" >/dev/null)
"$ws" mission accept -m "$slug" >/dev/null 2>&1 || true # step 1, if ws hasn't
"$ws" mission accept -m "$slug" >/dev/null              # step 2: read, and good
"$ws" land records "$slug" >/dev/null

# Step 3: ws starts c2 in demo, who makes the change in two commits and
# leaves two files uncommitted.
crew c2
demo="$dir/repos/demo/$slug"
mkdir -p "$demo/internal/greet" "$demo/docs"
printf 'package greet\n\n// To is the greeting for name, or for the world when there is none.\nfunc To(name string) string {\n\tif name == "" {\n\t\tname = "world"\n\t}\n\treturn "hello, " + name\n}\n' >"$demo/internal/greet/greet.go"
printf 'package greet\n\nimport "testing"\n\nfunc TestTo(t *testing.T) {\n\tif got := To("Ada"); got != "hello, Ada" {\n\t\tt.Errorf("To(Ada) = %%q", got)\n\t}\n\tif got := To(""); got != "hello, world" {\n\t\tt.Errorf("To() = %%q", got)\n\t}\n}\n' >"$demo/internal/greet/greet_test.go"
commit "$demo" "feat(greet): a greeting for a name, or for the world"
printf '\n## Greeting someone\n\n    go run . Ada\n' >>"$demo/README.md"
printf '# Greeting\n\n`greet.To` is the one place a greeting is made.\n' >"$demo/docs/greeting.md"
commit "$demo" "docs: how to greet someone"
(cd "$demo" && "$ws" WS_MISSION="$slug" WS_CREW=c2 mission done "greet.To greets by name and falls back to world, with a test; the README and docs say how" --handoff "$(handoff c2 '`internal/greet` with its test, in one commit; the README and `docs/greeting.md` in another.' '`main.go` still prints its own greeting: wiring it to `greet.To` is left uncommitted, half done.')" >/dev/null)
printf '\n// TODO: print greet.To(os.Args[1:]...)\n' >>"$demo/main.go"
printf 'scratch: what go run . printed\nhello, world\n' >"$demo/scratch.txt"
"$ws" mission accept -m "$slug" >/dev/null 2>&1 || true # step 3, if ws hasn't

# A note of the mission's own, which no capsule made.
note="$dir/.ws/missions/$slug/files/open-questions.md"
printf '# Open questions\n\n- Should a name with spaces be quoted?\n- Is world the right fallback?\n' >"$note"
"$ws" mission artifact add -m "$slug" "$note" "Open questions" --kind note >/dev/null 2>&1 || true

cat <<EOF
work: the mission $slug, at step 4 of 6, a Review that's yours.

  /missions/$slug?tab=work    its Work tab (in the web UI)
  $ws mission show $slug      the same mission, in the terminal

  records/$slug   landed: the note, c1's
  demo/$slug      two commits, and main.go and scratch.txt not committed; c2 is still there
  demo/later      lifted for step 6, untouched
EOF
