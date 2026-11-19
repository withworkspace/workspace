#!/usr/bin/env bash
# One command for trying this checkout's ws by hand: a fresh throwaway
# workspace in a temporary folder (scripts/test-workspace.sh builds it),
# seeded with what you want to look at, its daemon and web UI running on
# this checkout's build, and all of it torn down afterwards.
#
#   scripts/try.sh up [seed...]       build ws, make a workspace, seed it, start it, say where it is
#   scripts/try.sh restart            build ws again and restart the daemon on it (the link stays)
#   scripts/try.sh seed <seed>...     run more seeds in it
#   scripts/try.sh info [key]         where it is: dir, link, origin, port, token, ws (one key prints its value alone)
#   scripts/try.sh ls                 every try on this machine
#   scripts/try.sh seeds              the seeds there are
#   scripts/try.sh down [name...]     stop its daemon and remove its folder
#
# (just try <the same>.) restart, seed and info mean this checkout's try,
# or the one -n <name> names when it has several. down with no name tears
# down every try made from this checkout; down --all, every try there is.
#
# up takes:
#   -n <name>      its name, and its folder's (default: one made up)
#   --ttl <hours>  how long it may last (default 4; 0 for no limit)
#   --owner <pid>  the process it lasts as long as (default: see below)
#   --no-build     use bin/workspace as it is (restart takes this too)
#
# Where they go: /tmp/ws-try/<name>, or under $TRY_ROOT.
#
# The try's own ws is <dir>/.try/ws: this checkout's bin/workspace, run in
# the try as you. It drops the mission, crew and agent session of whoever
# calls it, which a ws run by crew would otherwise act for, and the shell
# wrapper's WS_ORIGIN_PWD, which would point it back at the workspace you
# came from. Always use it rather than bin/workspace. Leading NAME=value
# arguments are set for the command, as env(1) takes them:
#
#   <dir>/.try/ws mission show
#   <dir>/.try/ws WS_MISSION=slow-check WS_CREW=c1 mission done "..."
#
# Teardown doesn't wait to be asked. A try has an owner, the process it
# was made for: the nearest process above this one that's been running a
# minute or more (your shell, or the agent whose command this is). A
# watcher started with the try tears it down once its owner has gone or
# its time is up, whichever's first, and up and ls sweep away any the
# watcher missed.
#
# Seeds are scripts/seeds/<name>.sh: each puts something particular in a
# try (a mission mid-check, say), and its second line says what, for
# `try.sh seeds`. One is run by bash in the try's folder, after the
# workspace is built, with:
#
#   TRY_DIR   the try's folder (the workspace's root)
#   TRY_WS    its ws (as above)
#   TRY_REPO  this checkout
#
# What a seed prints is shown after the try's details: say what's there
# to look at, and where.
#
# A seed that wants crew to look like an agent's without running one puts
# a stand-in for its CLI in $TRY_DIR/.try/bin, before its first ws
# command: that folder is first on the PATH of the try's ws, and so of
# the daemon it starts and the crew that daemon launches.
set -euo pipefail

# All of it runs from main, called on this file's last line: bash reads a
# script as it goes, and the watcher outlives edits to this one.
main() {
	here="$(cd "$(dirname "$0")/.." && pwd -P)"
	bin="$here/bin/workspace"
	root="${TRY_ROOT:-/tmp/ws-try}"
	[ -d "$root" ] && root="$(cd "$root" && pwd -P)"

	local cmd="${1:-}"
	[ $# -gt 0 ] && shift
	case "$cmd" in
	up) cmd_up "$@" ;;
	down) cmd_down "$@" ;;
	restart) cmd_restart "$@" ;;
	seed) cmd_seed "$@" ;;
	info) cmd_info "$@" ;;
	ls) cmd_ls ;;
	seeds) cmd_seeds ;;
	_reap) reap "$1" ;;
	_owner) owner_lines "$1" ;;
	"" | -h | --help | help) usage ;;
	*)
		echo "try.sh: no such command: $cmd" >&2
		usage >&2
		exit 2
		;;
	esac
}

# The comment this file opens with.
usage() { sed -n '2,/^set -euo/p' "$here/scripts/try.sh" | sed -e '$d' -e 's/^# \{0,1\}//'; }

die() {
	echo "try.sh: $*" >&2
	exit 1
}

# --- what's kept about a try: <dir>/.try/info, a key and its value a line ---

get() { # <name> <key>
	[ -f "$root/$1/.try/info" ] || return 0
	sed -n "s/^$2=//p" "$root/$1/.try/info" | head -n 1
}

put() { # <name> <key> <value>
	local f="$root/$1/.try/info"
	{
		grep -v "^$2=" "$f" 2>/dev/null || true
		printf '%s=%s\n' "$2" "$3"
	} >"$f.new"
	mv -f "$f.new" "$f"
}

# Every try's name, oldest first.
names() {
	[ -d "$root" ] || return 0
	local d
	for d in "$root"/*/; do
		[ -f "$d.try/info" ] && printf '%s %s\n' "$(sed -n 's/^created=//p' "$d.try/info")" "$(basename "$d")"
	done | sort -n | cut -d' ' -f2
}

# The names of the tries made from this checkout.
mine() {
	local n
	for n in $(names); do
		[ "$(get "$n" repo)" = "$here" ] && echo "$n"
	done
	return 0
}

# The one try a command means: the one named, or this checkout's only one.
one() { # [name]
	if [ -n "${1:-}" ]; then
		[ -f "$root/$1/.try/info" ] || die "no try named $1 in $root (try.sh ls lists them)"
		echo "$1"
		return
	fi
	local m
	m="$(mine)"
	case "$(printf '%s' "$m" | grep -c . || true)" in
	0) die "this checkout has no try: try.sh up makes one" ;;
	1) echo "$m" ;;
	*) die "this checkout has several tries ($(echo $m)): say which with -n <name>" ;;
	esac
}

# --- processes ---

# How long a process has been running, in seconds, from ps's etime
# ([[dd-]hh:]mm:ss).
age_of() { # <pid>
	local t d=0 h=0 m=0 s=0 a b c
	t="$(ps -o etime= -p "$1" 2>/dev/null | tr -d ' ')"
	[ -n "$t" ] || {
		echo 0
		return
	}
	case "$t" in *-*)
		d="${t%%-*}"
		t="${t#*-}"
		;;
	esac
	IFS=: read -r a b c <<<"$t"
	if [ -n "${c:-}" ]; then h="$a" m="$b" s="$c"; else m="$a" s="$b"; fi
	echo $((10#$d * 86400 + 10#$h * 3600 + 10#$m * 60 + 10#$s))
}

# When a process started, as ps says it: with its pid, which process it is
# (a pid alone is given out again). ps writes it in the caller's locale
# and time zone, and a try is looked at by more than whoever made it (a
# terminal's shell, crew with no LANG), so it's asked for in one of each.
started() { LC_ALL=C TZ=UTC ps -o lstart= -p "$1" 2>/dev/null | sed 's/  */ /g; s/^ //; s/ $//'; }

# Whether a process has gone: nothing has its pid, or something that
# started at another time has. A ps that can't see init can't say who's
# there (in a sandbox it sees only its own), and nothing has gone by it.
gone() { # <pid> [when it started]
	local now
	now="$(started "$1")"
	if [ -z "$now" ]; then
		[ -n "$(started 1)" ]
		return
	fi
	[ -n "${2:-}" ] && [ "$now" != "$2" ]
}

# What's kept about a try's owner. Not as owner and owner_started, which
# is what copies of this script in other checkouts once kept, and still
# read: they compare the start time with their own locale's, and tore
# down tries whose owners were there. Finding neither, they leave it be.
owner_lines() { printf 'owner_pid=%s\nowner_began=%s\n' "$1" "$(started "$1")"; }

# A try's owner's pid: one made by such a copy has it as owner.
owner_of() { # <name>
	local owner
	owner="$(get "$1" owner_pid)"
	[ -n "$owner" ] || owner="$(get "$1" owner)"
	echo "${owner:-0}"
}

# A try's owner, when nobody says: the nearest process above this one
# that's been running a minute or more. The shells and the just between
# it and this script were started for this command, and end with it.
find_owner() {
	local p="$PPID"
	while [ "${p:-0}" -gt 1 ]; do
		if [ "$(age_of "$p")" -ge 60 ]; then
			echo "$p"
			return
		fi
		p="$(ps -o ppid= -p "$p" 2>/dev/null | tr -d ' ')"
	done
	echo 0
}

# The daemons running for a try's workspace: the processes whose command
# ends as a daemon's for it does. What's looked for goes to awk in its
# environment: in its arguments, awk's own command would end the same way.
daemon_pids() { # <dir>
	ps -axo pid=,command= | TRY_WANT=" daemon run --root $1" awk '
		{ want = ENVIRON["TRY_WANT"] }
		length($0) >= length(want) && substr($0, length($0) - length(want) + 1) == want { print $1 }'
}

# --- teardown ---

# Why a try's time is up, or nothing if it isn't.
stale() { # <name>
	local owner expires
	owner="$(owner_of "$1")"
	expires="$(get "$1" expires)"
	# owner_began is empty for a try an older copy made: its start time is
	# in a locale nobody kept, so that one's owner has gone when its pid has.
	if [ "$owner" -gt 0 ] && gone "$owner" "$(get "$1" owner_began)"; then
		echo "its owner (pid $owner) has gone"
	elif [ "${expires:-0}" -gt 0 ] && [ "$(date +%s)" -ge "$expires" ]; then
		echo "its time was up"
	fi
}

# Stop a try's daemon (and so its crew), and remove its folder. Only a
# folder try.sh made is removed: one under its root, holding .try/info.
teardown() { # <name>
	local name="$1" dir="$root/$1" ws pids pid i
	case "$name" in "" | */* | .*) die "not a try's name: $name" ;; esac
	[ -f "$dir/.try/info" ] || die "no try named $name in $root (try.sh ls lists them)"
	ws="$dir/.try/ws"
	# Asked first, so it hangs up its crew and closes its records; given
	# five seconds, as a ws that's gone or a daemon that's stuck won't answer.
	if [ -x "$ws" ] && [ -x "$(get "$name" bin)" ] && [ -n "$(daemon_pids "$dir")" ]; then
		"$ws" daemon stop >/dev/null 2>&1 &
		pid=$!
		for i in $(seq 1 50); do
			kill -0 "$pid" 2>/dev/null || break
			sleep 0.1
		done
		kill "$pid" 2>/dev/null || true
		wait "$pid" 2>/dev/null || true
	fi
	pids="$(daemon_pids "$dir")"
	if [ -n "$pids" ]; then
		kill $pids 2>/dev/null || true
		for i in $(seq 1 30); do
			[ -z "$(daemon_pids "$dir")" ] && break
			sleep 0.1
		done
		pids="$(daemon_pids "$dir")"
		[ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
	fi
	rm -rf "$dir"
	[ -e "$dir" ] && die "couldn't remove $dir"
	return 0
}

# Tear down the tries whose time is up, saying which.
sweep() {
	local n why
	for n in $(names); do
		why="$(stale "$n")"
		[ -n "$why" ] || continue
		teardown "$n" && echo "Tore down $n: $why." >&2
	done
	return 0
}

# The watcher: tears a try down once its time is up, and goes once the
# try has.
reap() { # <name>
	while [ -f "$root/$1/.try/info" ]; do
		if [ -n "$(stale "$1")" ]; then
			teardown "$1" >/dev/null 2>&1 || true
			return 0
		fi
		sleep 15
	done
}

# Start the watcher, in a session of its own: it outlives this command,
# and the shell or agent that ran it.
watch() { # <name>
	if command -v perl >/dev/null 2>&1; then
		TRY_ROOT="$root" perl -MPOSIX -e 'fork and exit; setsid(); exec @ARGV' \
			"$here/scripts/try.sh" _reap "$1" </dev/null >/dev/null 2>&1
	else
		TRY_ROOT="$root" nohup "$here/scripts/try.sh" _reap "$1" </dev/null >/dev/null 2>&1 &
	fi
}

# --- the try's ws ---

# A string as sh reads it back, whatever's in it.
quoted() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }

write_ws() { # <dir>
	cat >"$1/.try/ws" <<EOF
#!/bin/sh
# This try's ws: this build, run here as you (scripts/try.sh says more).
dir=$(quoted "$1")
bin=$(quoted "$bin")
# Whose mission, crew or conversation a command acts for, and where the
# shell wrapper says it was run from.
unset WS_MISSION WS_CREW WS_ROLE WS_RUN WS_CONVERSATION WS_ORIGIN_PWD
# The agent session the caller may be in (internal/crew/env.go's
# agentSessionEnv): a daemon started here would hand it to its agents.
unset CLAUDECODE CLAUDE_PID CLAUDE_EFFORT CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH
unset CLAUDE_CODE_SESSION_ID CLAUDE_CODE_CHILD_SESSION CLAUDE_CODE_SESSION_ATTENDED
unset CLAUDE_CODE_MESSAGING_SOCKET CLAUDE_CODE_MESSAGING_TOKEN
unset AI_AGENT PI_CODING_AGENT PI_SESSION_ID PI_SESSION_FILE PI_PROVIDER PI_MODEL PI_REASONING_LEVEL
unset AGENT OPENCODE OPENCODE_PID OPENCODE_CLIENT
# What a seed stands in for a program with (an agent's CLI, say), which
# the daemon a command here starts finds first.
PATH="\$dir/.try/bin:\$PATH"
export PATH
while [ \$# -gt 0 ]; do
	case "\$1" in
	[A-Za-z_]*=*) export "\$1" && shift ;;
	*) break ;;
	esac
done
[ -x "\$bin" ] || { echo "no ws at \$bin: run just build there" >&2; exit 1; }
case "\$(pwd -P)/" in "\$dir"/*) ;; *) cd "\$dir" ;; esac
exec "\$bin" "\$@"
EOF
	chmod +x "$1/.try/ws"
}

# --- seeds ---

seed_file() { echo "$here/scripts/seeds/$1.sh"; }

check_seeds() {
	local s
	for s in "$@"; do
		[ -f "$(seed_file "$s")" ] || die "no seed named $s (try.sh seeds lists them)"
	done
}

# Run seeds in a try, keeping what each says.
run_seeds() { # <name> <seed>...
	local name="$1" dir="$root/$1" s had
	shift
	for s in "$@"; do
		echo "Seeding $s..." >&2
		[ -s "$dir/.try/said" ] && echo >>"$dir/.try/said"
		# What it prints is kept to show; what its commands say as they
		# go (stderr) is only wanted if it fails.
		if ! (cd "$dir" && TRY_DIR="$dir" TRY_WS="$dir/.try/ws" TRY_REPO="$here" bash "$(seed_file "$s")") >>"$dir/.try/said" 2>"$dir/.try/seed.log"; then
			cat "$dir/.try/seed.log" >&2
			die "the seed $s failed"
		fi
		had="$(get "$name" seeds)"
		put "$name" seeds "${had:+$had }$s"
	done
}

cmd_seeds() {
	local f
	for f in "$here"/scripts/seeds/*.sh; do
		[ -f "$f" ] || continue
		printf '%-14s %s\n' "$(basename "$f" .sh)" "$(sed -n '2s/^# *//p' "$f")"
	done
}

# --- commands ---

build() {
	command -v just >/dev/null 2>&1 || die "just isn't installed: build bin/workspace yourself, then pass --no-build"
	echo "Building ws..." >&2
	(cd "$here" && just build >&2) || die "just build failed"
}

# Start a try's daemon, if it isn't up, and keep where its web UI is.
start() { # <name>
	local name="$1" link origin
	link="$("$root/$name/.try/ws" web --print)" || die "the daemon didn't start (see $root/$name/.ws/daemon.log)"
	origin="${link%%/auth\?*}"
	put "$name" link "$link"
	put "$name" origin "$origin"
	put "$name" port "${origin##*:}"
	put "$name" token "$(printf '%s' "$link" | sed -n 's/.*[?&]t=\([^&]*\).*/\1/p')"
}

cmd_up() {
	local name="" ttl=4 owner="" nobuild="" seeds=() dir now expires
	while [ $# -gt 0 ]; do
		case "$1" in
		-n | --name)
			name="${2:-}"
			shift 2 || die "-n takes a name"
			;;
		--ttl)
			ttl="${2:-}"
			shift 2 || die "--ttl takes a number of hours"
			;;
		--owner)
			owner="${2:-}"
			shift 2 || die "--owner takes a pid"
			;;
		--no-build)
			nobuild=1
			shift
			;;
		-*) die "up doesn't take $1" ;;
		*)
			seeds+=("$1")
			shift
			;;
		esac
	done
	case "$ttl" in "" | *[!0-9]*) die "--ttl takes a whole number of hours, not $ttl" ;; esac
	[ -n "$name" ] || name="t$(printf '%04x%04x' "$RANDOM" "$RANDOM")"
	case "$name" in [a-z0-9]*) ;; *) die "a try's name starts with a letter or a number, not as $name does" ;; esac
	case "$name" in *[!a-z0-9-]*) die "a try's name is lower-case letters, numbers and dashes, not $name" ;; esac
	check_seeds ${seeds[@]+"${seeds[@]}"}
	if [ -z "$owner" ]; then
		owner="$(find_owner)"
	elif ! kill -0 "$owner" 2>/dev/null; then
		die "--owner $owner isn't a running process"
	fi

	mkdir -p "$root"
	root="$(cd "$root" && pwd -P)"
	dir="$root/$name"
	[ -e "$dir" ] && die "$dir is already there: try.sh down $name removes it"
	sweep
	[ -n "$nobuild" ] || build
	[ -x "$bin" ] || die "no ws at $bin: run just build"

	echo "Making a workspace..." >&2
	"$here/scripts/test-workspace.sh" "$dir" >/dev/null
	mkdir -p "$dir/.try"
	printf '/.try/\n' >>"$dir/.git/info/exclude"
	now="$(date +%s)"
	expires=0
	[ "$ttl" -gt 0 ] && expires=$((now + ttl * 3600))
	{
		printf 'name=%s\nrepo=%s\nbin=%s\ncreated=%s\nexpires=%s\n' "$name" "$here" "$bin" "$now" "$expires"
		owner_lines "$owner"
	} >"$dir/.try/info"
	write_ws "$dir"
	# From here there's something to leave behind: a try that doesn't come
	# up is torn down, not left half-made.
	trap 'teardown "'"$name"'" >/dev/null 2>&1 || true' EXIT
	[ ${#seeds[@]} -eq 0 ] || run_seeds "$name" "${seeds[@]}"
	start "$name"
	watch "$name"
	trap - EXIT

	echo >&2
	show "$name"
	if [ -s "$dir/.try/said" ]; then
		echo
		cat "$dir/.try/said"
	fi
	echo
	echo "Torn down (daemon stopped, folder removed) $(lasts "$name"), or now:"
	echo
	echo "  scripts/try.sh down $name"
}

# When a try is torn down without being asked, in words.
lasts() { # <name>
	local owner expires left what=""
	owner="$(owner_of "$1")"
	expires="$(get "$1" expires)"
	if [ "$owner" -gt 0 ]; then
		what="when pid $owner ($(ps -o comm= -p "$owner" 2>/dev/null | sed 's:.*/::')) exits"
	fi
	if [ "${expires:-0}" -gt 0 ]; then
		left=$(((expires - $(date +%s) + 59) / 60))
		[ "$left" -lt 0 ] && left=0
		what="${what:+$what or }in $((left / 60))h$(printf '%02d' $((left % 60)))"
	fi
	echo "${what:-only when you say}"
}

show() { # <name>
	local name="$1" dir="$root/$1" state="stopped" seeds
	[ -n "$(daemon_pids "$dir")" ] && state="running"
	seeds="$(get "$name" seeds)"
	cat <<EOF
name    $name
dir     $dir
daemon  $state
link    $(get "$name" link)
origin  $(get "$name" origin)
port    $(get "$name" port)
token   $(get "$name" token)
ws      $dir/.try/ws
seeds   ${seeds:-none}
EOF
}

# The -n <name> a command was given, if any, then what's left of its
# arguments (in rest).
named=""
rest=()
take_name() {
	named=""
	rest=()
	while [ $# -gt 0 ]; do
		case "$1" in
		-n | --name)
			named="${2:-}"
			shift 2 || die "-n takes a name"
			;;
		*)
			rest+=("$1")
			shift
			;;
		esac
	done
}

cmd_info() {
	local name key
	take_name "$@"
	name="$(one "$named")"
	key="${rest[0]:-}"
	case "$key" in
	"") show "$name" ;;
	dir) echo "$root/$name" ;;
	ws) echo "$root/$name/.try/ws" ;;
	name | link | origin | port | token | seeds) get "$name" "$key" ;;
	*) die "info knows dir, link, origin, port, token, ws, seeds and name, not $key" ;;
	esac
}

cmd_restart() {
	local name nobuild="" a
	take_name "$@"
	for a in ${rest[@]+"${rest[@]}"}; do
		case "$a" in
		--no-build) nobuild=1 ;;
		*) die "restart doesn't take $a" ;;
		esac
	done
	name="$(one "$named")"
	[ -n "$nobuild" ] || build
	"$root/$name/.try/ws" daemon stop >&2
	start "$name"
	show "$name"
}

cmd_seed() {
	local name
	take_name "$@"
	[ ${#rest[@]} -gt 0 ] || die "seed takes a seed's name (try.sh seeds lists them)"
	check_seeds "${rest[@]}"
	name="$(one "$named")"
	: >"$root/$name/.try/said"
	run_seeds "$name" "${rest[@]}"
	cat "$root/$name/.try/said"
}

cmd_ls() {
	local n dir state from
	sweep
	[ -n "$(names)" ] || {
		echo "No tries in $root." >&2
		return 0
	}
	printf '%-14s %-8s %-24s %-22s %s\n' NAME DAEMON ORIGIN ENDS FROM
	for n in $(names); do
		dir="$root/$n"
		state="stopped"
		[ -n "$(daemon_pids "$dir")" ] && state="running"
		from="$(get "$n" repo)"
		[ "$from" = "$here" ] && from="here"
		printf '%-14s %-8s %-24s %-22s %s\n' "$n" "$state" "$(get "$n" origin)" "$(lasts "$n" | sed 's/^when //; s/ exits//; s/ (.*)//')" "$from"
	done
}

cmd_down() {
	local all="" which=() n
	while [ $# -gt 0 ]; do
		case "$1" in
		--all) all=1 ;;
		-n | --name) ;;
		-*) die "down doesn't take $1" ;;
		*) which+=("$1") ;;
		esac
		shift
	done
	if [ -n "$all" ]; then
		[ ${#which[@]} -eq 0 ] || die "down takes names or --all, not both"
		for n in $(names); do which+=("$n"); done
	elif [ ${#which[@]} -eq 0 ]; then
		for n in $(mine); do which+=("$n"); done
	fi
	if [ ${#which[@]} -eq 0 ]; then
		echo "Nothing to tear down." >&2
		return 0
	fi
	for n in "${which[@]}"; do
		teardown "$n"
		echo "Tore down $n." >&2
	done
}

main "$@"; exit $?
