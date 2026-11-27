#!/usr/bin/env bash
# Cache: tasks whose recipe names their inputs, reused rather than run.
#
# For looking at cached tasks (ws run, ws run cache): a pass is kept with
# its outputs, and a run asked for with the same inputs (here or in
# another capsule) is that pass, reused, its outputs put back.
#
#   scripts/try.sh up cache
#
# It lifts two capsules of demo, cache-one and cache-two, whose
# ws.repo.toml has:
#
#   build   go build into dist/, cached: inputs are the Go files and
#           go.mod, outputs dist
#   gen     cached too, but writes notes.txt, which its outputs don't
#           name: it's told so
#
# It runs build twice in cache-one (run, then reused) and once in
# cache-two (reused from cache-one's), and gen once. It asks no agent
# anything.
set -euo pipefail

ws="$TRY_WS"
dir="$TRY_DIR"

for c in cache-one cache-two; do
	if [ -d "$dir/repos/demo/$c" ]; then
		echo "$dir already has the capsule demo/$c" >&2
		exit 1
	fi
	"$ws" lift demo "$c" >/dev/null
	capsule="$dir/repos/demo/$c"
	cat >>"$capsule/ws.repo.toml" <<'TOML'

[run.task.build]
command = "mkdir -p dist && go build -o dist/demo . && echo built dist/demo"
inputs = ["**/*.go", "go.mod"]
outputs = ["dist"]

[run.task.gen]
command = "mkdir -p gen && echo 'package gen' > gen/gen.go && date > notes.txt && echo generated"
inputs = ["go.mod"]
outputs = ["gen"]
TOML
	printf 'dist/\ngen/\nnotes.txt\n' >>"$capsule/.gitignore"
	git -C "$capsule" add -A
	git -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false -C "$capsule" commit -q -m "Cached build"
done

one="$dir/repos/demo/cache-one"
two="$dir/repos/demo/cache-two"
(cd "$one" && "$ws" run build >/dev/null 2>&1) || true
(cd "$one" && "$ws" run build >/dev/null 2>&1) || true
(cd "$two" && "$ws" run build >/dev/null 2>&1) || true
(cd "$one" && "$ws" run gen >/dev/null 2>&1) || true

cat <<EOF
cache: cached tasks, reused rather than run.

  /runs                            build in cache-one (run 2 reused, run 1 behind it, kept), build in cache-two (reused from cache-one's), gen (kept, notes.txt not in its outputs)
  cd $two
  $ws run build                    reused: prints what the pass printed, and says which run it was
  $ws run --fresh build            run anyway, and kept in its place
  $ws run show build               reused from, its outputs
  $ws run cache                    what's kept
  $ws run cache clear              drop it all
EOF
