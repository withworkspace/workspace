#!/usr/bin/env bash
# An artifact that's more than its page, in two versions.
#
# A release notes page, by you, added as a folder: its page, a second
# page it links to, a stylesheet, a module and what it imports, data the
# module fetches, and two images (one that isn't text). The second
# version is the first checked out, with its stylesheet changed and a
# file added: what wasn't touched is the same files. It asks no agent
# anything, so it's the same every time.
#
#   scripts/try.sh up files
set -euo pipefail

ws="$TRY_WS"
from="$TRY_REPO/scripts/seeds/files"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cp -R "$from/site" "$work/site"
# A file that isn't text: a one-pixel PNG.
base64 -d >"$work/site/img/dot.png" <<<"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

# Its ref is what ws says it is: another seed may have added one first.
ref="$("$ws" artifact add "$work/site" --title "Release notes page" \
	--note "A page of releases, read from a file of its own, with a second page beside it. Its tweaks: how tight, and whether each change says its kind." 2>/dev/null)"
id="${ref%%.*}"

# The next version, as an agent would make it: v1's files in a folder,
# two of them changed, one added.
"$ws" artifact checkout "$id" "$work/next" >/dev/null 2>&1
sed -i.bak 's/--accent: #3b6fd4;/--accent: #b4532a;/; s/--accent: #7ea6f5;/--accent: #f0a37c;/' "$work/next/css/site.css"
rm "$work/next/css/site.css.bak"
sed -i.bak 's/"version": "0.8.0"/"version": "0.8.0 (next)"/' "$work/next/data/releases.json"
rm "$work/next/data/releases.json.bak"
printf 'Release notes page: the accent is warm in this version.\n' >"$work/next/notes.txt"
"$ws" artifact add "$work/next" --to "$id" \
	--note "A warmer accent, and the next release marked as such. The page, the scripts and the images are as they were." >/dev/null

cat <<EOF2
files: an artifact that's more than its page.

  /artifacts/$id                    the second version, running: its stylesheet, module, data and images are its own files
  /artifacts/$id/v1                 the first; Source (s) lists its files, to read each
  /artifacts/$id/compare?v=1,2      Source diff: the three files that differ
  ws artifact show $id --files      its files; ws artifact checkout $id puts them in a folder
EOF2
