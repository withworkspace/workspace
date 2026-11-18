#!/usr/bin/env bash
# Four missions, each at a Wait step, one for each way a Wait is passed.
#
# A Wait says who it's on, and nothing passes it by itself unless it has a
# check. This seed has one of each, to look at side by side: it asks no
# agent anything, so it's the same every time.
#
#   scripts/try.sh up waits
#
#   cut-the-tag       on you, no check: Manual. Yours, under Needs you.
#   tag-and-publish   on you, with a check: Watched. Yours, and ws looks too.
#   on-the-registry   on the release workflow, with a check: Watched. Under Waiting.
#   platform-deploy   on the platform team, no check: Manual. Under Waiting.
#
# The checks look for a file in the try's folder, so making it is what
# they wait for: `touch $TRY_DIR/tagged` passes tag-and-publish at ws's
# next look (within a minute, or Look now), `touch $TRY_DIR/published`
# on-the-registry.
set -euo pipefail

ws="$TRY_WS"

# wait_mission <title> <slug> <the Wait's words> <its parameters>: a
# mission whose plan is a Review, the Wait, and a Work step after it, with
# the Review passed, so it's at the Wait.
wait_mission() {
	local title="$1" slug="$2" words="$3" params="$4"
	if [ -d ".ws/missions/$slug" ]; then
		echo "$TRY_DIR already has the mission $slug" >&2
		exit 1
	fi
	"$ws" mission new "$title" --brief "## Goal
$title, then carry on from what was released.

## Context
- Seeded by the workspace repo's scripts/seeds/waits.sh.
" >/dev/null
	"$ws" WS_MISSION="$slug" lift demo "$slug" >/dev/null
	"$ws" mission plan -m "$slug" --set - >/dev/null <<EOF
## Z
- It's released, and demo is moved on to it.
## Stops early
- None.
## Steps
1. Review: **Ready to release.** The change, before it's released.
2. Wait: $words
$params
3. Work: **Move on to it.** demo requires the released version.
   - in: \`demo/$slug\`
EOF
	"$ws" mission accept -m "$slug" >/dev/null 2>&1
}

wait_mission "Cut the tag" cut-the-tag \
	"**Cut the tag.** The user tags a release of demo, and it's published." \
	"   - on: you"
wait_mission "Tag and publish" tag-and-publish \
	"**Tag and publish.** The user tags a release of demo, and it's published." \
	"   - on: you
   - check: \`test -f tagged\` passes"
wait_mission "On the registry" on-the-registry \
	"**On the registry.** The release workflow publishes the version to the registry." \
	"   - on: the release workflow
   - check: \`test -f published\` passes"
wait_mission "Platform deploy" platform-deploy \
	"**Platform deploy.** The platform team deploys the token service to prd." \
	"   - on: the platform team"

cat <<EOF
waits: four missions, each at step 2 of 3, a Wait.

  /missions/cut-the-tag       on you, Manual: Mark done passes it
  /missions/tag-and-publish   on you, Watched: touch $TRY_DIR/tagged, and ws passes it (or Mark done, and it looks)
  /missions/on-the-registry   on the release workflow, Watched: touch $TRY_DIR/published, then Look now
  /missions/platform-deploy   on the platform team, Manual: Mark done, once it's happened
EOF
