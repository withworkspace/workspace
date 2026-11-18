#!/usr/bin/env bash
# A mission at its Review step: step 1's work done by a shell standing in
# for crew, committed, its check met. For the workspace scripts/try.sh
# makes (its repo demo, a daemon running).
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"
slug="greet-by-name"

bash "$(dirname "$0")/../ready-to-start/seed.sh"

# Crew here may be a shell: c1 is one, with no agent to pay for.
if ! grep -q '"shell"' ws.local.toml; then
	sed -i.bak 's/^allowed = \[\(.*\)\]$/allowed = [\1, "shell"]/' ws.local.toml && rm -f ws.local.toml.bak
fi
"$ws" mission dispatch -m "$slug" --harness shell --capsule "demo/$slug" "A shell, standing in for crew." >/dev/null
for _ in $(seq 1 50); do
	"$ws" show -m "$slug" -f llm c1 >/dev/null 2>&1 && break
	sleep 0.2
done

# The work, as crew would leave it.
capsule="$dir/repos/demo/$slug"
cat >"$capsule/main.go" <<'GO'
package main

import (
	"fmt"
	"os"
)

func greeting(name string) string {
	if name == "" {
		name = "world"
	}
	return "hello, " + name
}

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(greeting(name))
}
GO
cat >"$capsule/main_test.go" <<'GO'
package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting("Ada"); got != "hello, Ada" {
		t.Errorf("greeting(Ada) = %q", got)
	}
	if got := greeting(""); got != "hello, world" {
		t.Errorf("greeting() = %q", got)
	}
}
GO
git -C "$capsule" add -A
git -C "$capsule" -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false commit -q -m "Greet by name"

handoff="$dir/.ws/missions/$slug/files/seed-handoff.md"
mkdir -p "$(dirname "$handoff")"
printf '## Done\n- greeting takes a name, and falls back to world, with a test.\n## Learned\n- Nothing.\n## Left\n- Nothing.\n## Read first\n- main.go\n' >"$handoff"
(cd "$capsule" && "$ws" WS_MISSION="$slug" WS_CREW=c1 mission done "Greets by name, with a test" --handoff "$handoff" >/dev/null)

# ws runs the step's check, and with it met, passes the step.
for _ in $(seq 1 150); do
	"$ws" show -m "$slug" -f llm 2>/dev/null | grep -q 'Step 2 of 3' && exit 0
	sleep 0.4
done
echo "step 1 wasn't passed: $("$ws" show -m "$slug" -f llm 2>&1 | sed -n 4,5p)" >&2
exit 1
