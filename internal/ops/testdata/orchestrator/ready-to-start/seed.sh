#!/usr/bin/env bash
# A mission ready to start: a brief, a capsule, a confirmed plan, and no
# crew yet. For the workspace scripts/try.sh makes (its repo demo).
#
# The workspace's process is put plainly, so there's one right thing to
# do: the try's own says more than a fix is proposed first, which this
# mission's plan doesn't do, and an orchestrator may fairly stop at that.
set -euo pipefail

ws="$TRY_WS"
slug="greet-by-name"

cat >PROCESS.md <<'MD'
# How we work

A mission's plan, once you've confirmed it, is what gets built: crew do
its Work steps as they're written. We always review code ourselves before
a pull request is opened.
MD

"$ws" mission new "Greet by name" --brief "## Goal
demo greets whoever it's told to: \`go run . Ada\` prints hello, Ada.

## Context
- Seeded for an eval of the orchestrator.
" >/dev/null

"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null

"$ws" mission plan -m "$slug" --set - >/dev/null <<PLAN
## Z
- \`go run . Ada\` prints hello, Ada, with a test, and its pull request is open.
## Stops early
- None.
## Steps
1. Work: **Greet by name.** greeting takes a name, and falls back to world.
   - in: \`demo/$slug\`
   - check: \`go test ./...\` passes
2. Review: **Read it.** The change, before its pull request.
3. Open Pull Request: **Open it.** From \`demo/$slug\`.
PLAN
