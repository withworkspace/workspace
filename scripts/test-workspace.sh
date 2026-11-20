#!/usr/bin/env bash
# Builds a throwaway ws workspace for trying missions, the web UI and agents
# by hand, with Claude Code (on haiku, to keep it cheap and quick) in every
# role: crew, briefing and the orchestrator.
#
#   scripts/test-workspace.sh [dir]     (default: ~/Projects/ws-test)
#
# It has two repos, whose "remotes" are local repos kept inside the
# workspace (.remotes/), so lift, dock and fetch work offline:
#
#   demo      a tiny Go program, for crew to change
#   records   proposals, ADRs and notes ([missions] records = "records"), each a kind
#             of document ([documents.adr], .proposal, .note), and where a
#             promoted artifact lives ([artifacts], in records:artifacts/).
#             It has no pull requests (landing = "direct"): a capsule's
#             commits go straight to main with ws land, or from its review
#             in the web UI.
#
# The GitHub org is made up. A file stands in for GitHub ([github] fixture
# in ws.local.toml, .remotes/github.json): demo has open pull requests
# there (a stack of three, a few on their own, one of yours), whose
# branches are in its remote, so the web UI's Reviews page has something
# to list, review and submit. Both repos have commits on main from the
# last few days (two merged pull requests on demo, commits pushed straight
# to records), so the Team page has something to show. Nothing leaves the machine: a review you
# submit is written into that file. Delete the folder and run this again
# for a fresh one.
set -euo pipefail

dir="${1:-$HOME/Projects/ws-test}"
if [ -e "$dir" ] && [ -n "$(ls -A "$dir" 2>/dev/null)" ]; then
	echo "$dir already exists and isn't empty: remove it first for a fresh test workspace" >&2
	exit 1
fi
mkdir -p "$dir"
dir="$(cd "$dir" && pwd)"

# Seed commits are made by "ws test" and never signed, whatever the global
# git config says (a signing agent would otherwise be asked for each one).
git_seed() { git -c user.name="ws test" -c user.email="ws-test@example.com" -c commit.gpgsign=false "$@"; }

# A "remote" for a repo: a plain repo with one commit holding the files the
# caller wrote into it.
make_remote() {
	local name="$1" src="$dir/.remotes/$1"
	git_seed -C "$src" init -q --initial-branch=main
	git_seed -C "$src" add -A
	git_seed -C "$src" commit -q -m "initial $name"
}

# A pull request's branch in demo's remote: branch <name> off <from>, by
# <author>, with main.go as given on stdin committed as <subject>, and
# GitHub's refs/pull/<number>/head pointing at it.
pr_branch() {
	local number="$1" name="$2" from="$3" author="$4" subject="$5" src="$dir/.remotes/demo"
	git -C "$src" checkout -q -B "$name" "$from"
	cat >"$src/main.go"
	pr_commit "$number" "$author" "$subject"
}

# Commit what's changed in demo's remote on the branch checked out, by
# <author>, and point the pull request's ref at it.
pr_commit() {
	local number="$1" author="$2" subject="$3" src="$dir/.remotes/demo"
	git -C "$src" add -A
	git -c user.name="$author" -c user.email="${author%%\[*}@example.com" -c commit.gpgsign=false -C "$src" commit -q -m "$subject"
	git -C "$src" update-ref "refs/pull/$number/head" HEAD
}

sha() { git -C "$dir/.remotes/demo" rev-parse "$1"; }

# Days ago, as dates: what's happened here is dated back from today, so
# the Reviews and Team pages have something recent whenever this is run.
day() { date -u -v-"$1"d +%Y-%m-%d 2>/dev/null || date -u -d "$1 days ago" +%Y-%m-%d; }
d1="$(day 1)" d2="$(day 2)" d3="$(day 3)" d4="$(day 4)" d5="$(day 5)"

# A commit on a remote's main: by <author>, at <when>, of whatever's
# changed there.
landed() {
	local repo="$1" author="$2" when="$3" subject="$4" src="$dir/.remotes/$1"
	git -C "$src" add -A
	GIT_AUTHOR_DATE="$when" GIT_COMMITTER_DATE="$when" git -c user.name="$author" -c user.email="$author@example.com" -c commit.gpgsign=false -C "$src" commit -q -m "$subject"
}

# What's landed on demo's main lately, for the Team page: two pull
# requests squashed in (the file that answers for GitHub has them as
# merged), and a commit pushed straight there.
seed_landed_demo() {
	local src="$dir/.remotes/demo"
	printf '# demo\n\nSays hello.\n' >"$src/README.md"
	landed demo alice "${d4}T11:00:00Z" "Say what demo is in a README (#8)"
	printf 'test:\n\tgo test ./...\n' >"$src/Makefile"
	landed demo bob "${d3}T15:30:00Z" "Add a Makefile (#9)"
	printf '# demo\n\nSays hello, to the world.\n' >"$src/README.md"
	landed demo carol "${d2}T09:45:00Z" "Fix the README's first line"
}

# What's landed on records' main lately: it takes commits straight to
# main, with no pull requests.
seed_landed_records() {
	local src="$dir/.remotes/records"
	printf '# Greeting in other languages\n\nWhich languages, and who translates them.\n' >"$src/notes/greeting-languages.md"
	landed records carol "${d2}T14:10:00Z" "Add a note on greeting in other languages"
	printf '# ADR 0001: One binary\n\n## Context\n\ndemo is one program.\n\n## Decision\n\nIt stays one.\n' >"$src/adr/0001-one-binary.md"
	landed records carol "${d1}T10:20:00Z" "Add ADR 0001: one binary"
	printf '# Shout mode\n\n## Problem\n\nSome greetings want to be loud.\n' >"$src/proposals/shout-mode.md"
	landed records dave "${d1}T16:05:00Z" "Propose a shout mode"
}

# A record written as documents are for GitHub, for the web UI's reader:
# front matter, an alert, maths, a diagram, an image beside it, and links
# to other records and to a heading.
seed_reading_records() {
	local src="$dir/.remotes/records"
	mkdir -p "$src/adr/img"
	cat >"$src/adr/img/loudness.svg" <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 80" width="240" height="80" font-family="sans-serif" font-size="11">
  <rect width="240" height="80" fill="#f4f4f6"/>
  <rect x="20" y="50" width="40" height="20" fill="#9c7fc5"/><text x="40" y="45" text-anchor="middle">hello</text>
  <rect x="100" y="30" width="40" height="40" fill="#6e509c"/><text x="120" y="25" text-anchor="middle">Hello!</text>
  <rect x="180" y="10" width="40" height="60" fill="#4b3470"/><text x="200" y="78" text-anchor="middle" fill="#fff">HELLO!</text>
</svg>
SVG
	cat >"$src/adr/0002-how-loud.md" <<'MD'
---
title: How loud a greeting is
status: proposed
author: carol
---

# ADR 0002: How loud a greeting is

> [!NOTE]
> This follows [ADR 0001](0001-one-binary.md): loudness is a flag of the one binary, not a second program.

## Context

[Shout mode](../proposals/shout-mode.md#problem) wants some greetings loud. How loud is a number, $L$, from the share of capitals $c$ and the marks $m$ after it.

## Decision

$$
L = 100 \cdot \frac{c + m}{1 + m}, \qquad 0 \le c \le 1
$$

A greeting is loud when $L \ge 50$. It costs nothing: $0 either way, and `$HOME` isn't read.

![Three greetings, each louder than the last](img/loudness.svg)

```mermaid
flowchart LR
  A[greeting] --> B{loud?}
  B -- yes --> C[HELLO!]
  B -- no --> D[hello]
```

> [!WARNING]
> Shouting in a terminal that's read by a script breaks the script.

## Consequences

- [x] One flag, `-loud`.
- [ ] A test of each side of 50.

See [the records' README](/README.md), [what's decided so far](#decision) and [the notes](../notes).
MD
	landed records carol "${d1}T17:40:00Z" "Add ADR 0002: how loud a greeting is"
}

# Open pull requests on demo, and the file that answers for GitHub about
# them (internal/github/fixture.go reads it).
seed_pull_requests() {
	local src="$dir/.remotes/demo"
	# A commit you reviewed on "GitHub" stays fetchable after a force-push,
	# as GitHub's do.
	git -C "$src" config uploadpack.allowAnySHA1InWant true

	# A stack of three, by alice: each builds on the last.
	pr_branch 11 alice/greet-by-name main alice "Greet by name" <<'GO'
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
	cat >"$src/main_test.go" <<'GO'
package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting("ada"); got != "hello, ada" {
		t.Errorf("greeting(ada) = %q", got)
	}
	if got := greeting(""); got != "hello, world" {
		t.Errorf("greeting() = %q", got)
	}
}
GO
	pr_commit 11 alice "Test greeting by name"

	pr_branch 12 alice/greet-languages alice/greet-by-name alice "Greet in other languages" <<'GO'
package main

import (
	"fmt"
	"os"
)

var hellos = map[string]string{
	"en": "hello",
	"fr": "bonjour",
	"es": "hola",
}

func greeting(name, lang string) string {
	if name == "" {
		name = "world"
	}
	return hellos[lang] + ", " + name
}

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(greeting(name, "en"))
}
GO
	cat >"$src/main_test.go" <<'GO'
package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting("ada", "en"); got != "hello, ada" {
		t.Errorf("greeting(ada, en) = %q", got)
	}
	if got := greeting("", "fr"); got != "bonjour, world" {
		t.Errorf("greeting(, fr) = %q", got)
	}
}
GO
	pr_commit 12 alice "Test greeting by language"

	pr_branch 13 alice/greet-flag alice/greet-languages alice "Pick the language with -lang" <<'GO'
package main

import (
	"flag"
	"fmt"
)

var hellos = map[string]string{
	"en": "hello",
	"fr": "bonjour",
	"es": "hola",
}

func greeting(name, lang string) string {
	if name == "" {
		name = "world"
	}
	return hellos[lang] + ", " + name
}

func main() {
	lang := flag.String("lang", "en", "the language to greet in")
	flag.Parse()
	fmt.Println(greeting(flag.Arg(0), *lang))
}
GO

	# On their own.
	pr_branch 15 bob/shout main bob "Add a shout mode" <<'GO'
package main

import (
	"fmt"
	"strings"
)

func greeting() string { return "hello" }

func shout(s string) string { return strings.ToUpper(s) + "!" }

func main() { fmt.Println(shout(greeting())) }
GO

	# One you asked for changes on, which has been pushed to since.
	pr_branch 19 dave/farewell main dave "Say goodbye too" <<'GO'
package main

import "fmt"

func greeting() string { return "hello" }

func farewell() string { return "bye" }

func main() {
	fmt.Println(greeting())
	fmt.Println(farewell())
}
GO
	local dave_reviewed
	dave_reviewed="$(sha dave/farewell)"
	pr_branch 19 dave/farewell dave/farewell dave "Spell goodbye out" <<'GO'
package main

import "fmt"

func greeting() string { return "hello" }

func farewell() string { return "goodbye" }

func main() {
	fmt.Println(greeting())
	fmt.Println(farewell())
}
GO

	# One with files in folders, a test beside what it tests, a lockfile
	# and a rename: for the review's list of files, and the order they're
	# read in.
	git -C "$src" checkout -q -B erin/render main
	mkdir -p "$src/internal/render/colour" "$src/cmd/demo" "$src/docs"
	cat >"$src/internal/render/render.go" <<'GO'
// Package render says hello.
package render

import "example.com/demo/internal/render/colour"

// Greeting is what demo prints.
func Greeting(name string) string {
	if name == "" {
		name = "world"
	}
	return colour.Green("hello") + ", " + name
}
GO
	cat >"$src/internal/render/render_test.go" <<'GO'
package render

import (
	"strings"
	"testing"
)

func TestGreeting(t *testing.T) {
	if got := Greeting("ada"); !strings.HasSuffix(got, ", ada") {
		t.Errorf("Greeting(ada) = %q", got)
	}
}
GO
	cat >"$src/internal/render/colour/colour.go" <<'GO'
// Package colour wraps words in terminal colours.
package colour

// Green is s in green.
func Green(s string) string { return "\033[32m" + s + "\033[0m" }
GO
	cat >"$src/cmd/demo/main.go" <<'GO'
package main

import (
	"fmt"
	"os"

	"example.com/demo/internal/render"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(render.Greeting(name))
}
GO
	git -C "$src" rm -q main.go
	git -C "$src" mv README.md docs/README.md
	printf 'example.com/colour v0.3.1 h1:9f2c1b7a6e5d4c3b2a19087f6e5d4c3b2a190=\nexample.com/colour v0.3.1/go.mod h1:0a1b2c3d4e5f60718293a4b5c6d7e8f901234=\n' >"$src/go.sum"
	pr_commit 20 erin "Move the greeting into a package"

	# A bot's, and a draft.
	git -C "$src" checkout -q -B dependabot/go-1.23 main
	printf 'module example.com/demo\n\ngo 1.23\n' >"$src/go.mod"
	pr_commit 17 "dependabot[bot]" "Bump go from 1.22 to 1.23"
	pr_branch 18 carol/colours main carol "WIP: colours" <<'GO'
package main

import "fmt"

const green = "\033[32m"

func greeting() string { return green + "hello" }

func main() { fmt.Println(greeting()) }
GO

	# Yours.
	pr_branch 16 you/version main "ws test" "Print a version with -v" <<'GO'
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func greeting() string { return "hello" }

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-v" {
		fmt.Println(version)
		return
	}
	fmt.Println(greeting())
}
GO
	git -C "$src" checkout -q main

	local url="https://github.com/example-org/demo/pull"
	cat >"$dir/.remotes/github.json" <<JSON
{
  "viewer": "you",
  "requested": [{"repo": "demo", "number": 15}],
  "repos": {
    "demo": {
      "prs": [
        {"number": 11, "title": "Greet by name", "authorLogin": "alice", "headRefName": "alice/greet-by-name", "baseRefName": "main", "headRefOid": "$(sha alice/greet-by-name)",
         "url": "$url/11", "statusRollup": "success", "reviewDecision": "APPROVED", "additions": 19, "deletions": 5, "changedFiles": 2,
         "createdAt": "${d3}T09:12:00Z", "updatedAt": "${d2}T15:40:00Z",
         "latest": [{"author": "you", "state": "APPROVED", "submittedAt": "${d2}T15:40:00Z", "commit": "$(sha alice/greet-by-name)"}]},
        {"number": 12, "title": "Greet in other languages", "authorLogin": "alice", "headRefName": "alice/greet-languages", "baseRefName": "alice/greet-by-name", "headRefOid": "$(sha alice/greet-languages)",
         "url": "$url/12", "statusRollup": "success", "reviewDecision": "REVIEW_REQUIRED", "additions": 12, "deletions": 6, "changedFiles": 2,
         "createdAt": "${d3}T09:30:00Z", "updatedAt": "${d1}T08:05:00Z",
         "requested": [{"login": "you"}]},
        {"number": 13, "title": "Pick the language with -lang", "authorLogin": "alice", "headRefName": "alice/greet-flag", "baseRefName": "alice/greet-languages", "headRefOid": "$(sha alice/greet-flag)",
         "url": "$url/13", "statusRollup": "pending", "reviewDecision": "REVIEW_REQUIRED", "additions": 5, "deletions": 7, "changedFiles": 1,
         "createdAt": "${d3}T10:02:00Z", "updatedAt": "${d1}T08:06:00Z",
         "requested": [{"login": "you"}]},
        {"number": 15, "title": "Add a shout mode", "authorLogin": "bob", "headRefName": "bob/shout", "baseRefName": "main", "headRefOid": "$(sha bob/shout)",
         "url": "$url/15", "statusRollup": "failure", "reviewDecision": "REVIEW_REQUIRED", "additions": 6, "deletions": 2, "changedFiles": 1,
         "createdAt": "${d2}T11:20:00Z", "updatedAt": "${d2}T11:20:00Z",
         "requested": [{"team": "demo-reviewers"}]},
        {"number": 19, "title": "Say goodbye too", "authorLogin": "dave", "headRefName": "dave/farewell", "baseRefName": "main", "headRefOid": "$(sha dave/farewell)",
         "url": "$url/19", "statusRollup": "success", "reviewDecision": "CHANGES_REQUESTED", "additions": 7, "deletions": 1, "changedFiles": 1,
         "createdAt": "${d4}T14:00:00Z", "updatedAt": "${d1}T07:30:00Z",
         "latest": [{"author": "you", "state": "CHANGES_REQUESTED", "submittedAt": "${d3}T10:00:00Z", "commit": "$dave_reviewed"}]},
        {"number": 20, "title": "Move the greeting into a package", "authorLogin": "erin", "headRefName": "erin/render", "baseRefName": "main", "headRefOid": "$(sha erin/render)",
         "url": "$url/20", "statusRollup": "success", "reviewDecision": "REVIEW_REQUIRED", "additions": 47, "deletions": 5, "changedFiles": 7,
         "createdAt": "${d2}T13:10:00Z", "updatedAt": "${d1}T09:20:00Z",
         "requested": [{"login": "you"}],
         "latest": [{"author": "bob", "state": "APPROVED", "submittedAt": "${d1}T09:20:00Z", "commit": "$(sha erin/render)"}]},
        {"number": 17, "title": "Bump go from 1.22 to 1.23", "authorLogin": "dependabot[bot]", "authorIsBot": true, "headRefName": "dependabot/go-1.23", "baseRefName": "main", "headRefOid": "$(sha dependabot/go-1.23)",
         "url": "$url/17", "statusRollup": "success", "additions": 1, "deletions": 1, "changedFiles": 1,
         "createdAt": "${d5}T03:00:00Z", "updatedAt": "${d5}T03:00:00Z"},
        {"number": 18, "title": "WIP: colours", "authorLogin": "carol", "isDraft": true, "headRefName": "carol/colours", "baseRefName": "main", "headRefOid": "$(sha carol/colours)",
         "url": "$url/18", "additions": 3, "deletions": 1, "changedFiles": 1,
         "createdAt": "${d2}T17:00:00Z", "updatedAt": "${d2}T17:00:00Z"},
        {"number": 16, "title": "Print a version with -v", "authorLogin": "you", "headRefName": "you/version", "baseRefName": "main", "headRefOid": "$(sha you/version)",
         "url": "$url/16", "statusRollup": "success", "reviewDecision": "APPROVED", "additions": 11, "deletions": 2, "changedFiles": 1,
         "createdAt": "${d2}T09:00:00Z", "updatedAt": "${d1}T06:45:00Z",
         "latest": [{"author": "alice", "state": "APPROVED", "submittedAt": "${d1}T06:45:00Z", "commit": "$(sha you/version)"}]}
      ],
      "merged": [
        {"number": 8, "title": "Say what demo is in a README", "authorLogin": "alice", "headRefName": "alice/readme", "baseRefName": "main",
         "url": "$url/8", "state": "MERGED", "additions": 3, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d5}T16:00:00Z", "updatedAt": "${d4}T11:00:00Z", "mergedAt": "${d4}T11:00:00Z",
         "latest": [{"author": "bob", "state": "APPROVED", "submittedAt": "${d4}T10:40:00Z"}]},
        {"number": 9, "title": "Add a Makefile", "authorLogin": "bob", "headRefName": "bob/makefile", "baseRefName": "main",
         "url": "$url/9", "state": "MERGED", "additions": 2, "deletions": 0, "changedFiles": 1,
         "createdAt": "${d4}T13:00:00Z", "updatedAt": "${d3}T15:30:00Z", "mergedAt": "${d3}T15:30:00Z",
         "latest": [{"author": "alice", "state": "CHANGES_REQUESTED", "submittedAt": "${d4}T17:00:00Z"}, {"author": "carol", "state": "APPROVED", "submittedAt": "${d3}T15:00:00Z"}]}
      ],
      "stacks": [
        {"number": 1, "base": "main", "open": true, "prs": [{"number": 11, "state": "open"}, {"number": 12, "state": "open"}, {"number": 13, "state": "open"}]}
      ],
      "bodies": {
        "11": "The greeting takes a name, and falls back to \"world\".\n\nBottom of a stack: languages and a flag follow.",
        "12": "Adds French and Spanish, picked by a second argument to \`greeting\`.\n\n- [x] tests\n- [ ] an unknown language should fall back to English: not done yet",
        "13": "Replaces the positional argument with \`-lang\`.",
        "15": "\`shout\` upper-cases the greeting. CI is red: the test still expects lower case.",
        "19": "Prints a farewell after the greeting. Now spelled out, as asked.",
        "16": "\`demo -v\` prints the version.",
        "20": "<!-- Say what it does and why. Delete this line. -->\nMoves \`greeting\` out of \`main.go\` into \`internal/render\`, so other commands can use it.\n\n- the greeting is green now, through a small \`colour\` package\n- \`main.go\` moves to \`cmd/demo\`\n- the README moves to \`docs/\`"
      },
      "said": {
        "12": [
          {"id": "C_demo_12_1", "author": "alice", "body": "This one's the middle of the stack: #13 swaps the argument for a flag.", "at": "${d3}T09:31:00Z", "url": "$url/12#issuecomment-1"},
          {"id": "R_demo_12_2", "author": "bob", "state": "COMMENTED", "body": "One question on the map, otherwise fine by me.", "at": "${d2}T16:10:00Z", "url": "$url/12#pullrequestreview-2", "commit": "$(sha alice/greet-languages)"}
        ],
        "19": [
          {"id": "R_demo_19_1", "author": "you", "state": "CHANGES_REQUESTED", "body": "Spell it out, then it's good.", "at": "${d3}T10:00:00Z", "url": "$url/19#pullrequestreview-1", "commit": "$dave_reviewed"},
          {"id": "C_demo_19_2", "author": "dave", "body": "Pushed: it says goodbye now.", "at": "${d1}T07:30:00Z", "url": "$url/19#issuecomment-2"}
        ],
        "20": [
          {"id": "C_demo_20_1", "author": "coverage[bot]", "bot": true, "body": "Coverage: 84.2% (+1.1%)", "at": "${d2}T13:14:00Z", "url": "$url/20#issuecomment-1"},
          {"id": "R_demo_20_2", "author": "carol", "state": "CHANGES_REQUESTED", "body": "The colour codes shouldn't be in the greeting itself: pipe it to a file and you get escape codes.", "at": "${d2}T15:00:00Z", "url": "$url/20#pullrequestreview-2", "commit": "$(sha main)"},
          {"id": "C_demo_20_3", "author": "erin", "body": "Fair. I'd rather do that in a follow-up: this one's already moving three things.", "at": "${d2}T15:20:00Z", "url": "$url/20#issuecomment-3"},
          {"id": "R_demo_20_4", "author": "bob", "state": "APPROVED", "body": "", "at": "${d1}T09:20:00Z", "url": "$url/20#pullrequestreview-4", "commit": "$(sha erin/render)"}
        ]
      },
      "threads": {
        "12": [
          {"id": "T_demo_12_1", "path": "main.go", "line": 18, "originalLine": 18, "side": "RIGHT",
           "comments": [
             {"author": "bob", "body": "question: what does this print for a language that isn't in the map?", "at": "${d2}T16:10:00Z", "url": "$url/12#discussion_r1",
              "commit": "$(sha alice/greet-languages)", "originalCommit": "$(sha alice/greet-languages)"},
             {"author": "alice", "body": "\", world\" for now. I'll fall back to English in a follow-up.", "at": "${d2}T16:25:00Z"}
           ]}
        ],
        "19": [
          {"id": "T_demo_19_1", "path": "main.go", "outdated": true, "originalLine": 7, "side": "RIGHT",
           "comments": [
             {"author": "you", "body": "nitpick: spell it out: \"goodbye\".", "at": "${d3}T10:00:00Z", "url": "$url/19#discussion_r1",
              "commit": "$dave_reviewed", "originalCommit": "$dave_reviewed", "diffHunk": "@@ -4,4 +4,9 @@\n func greeting() string { return \"hello\" }\n \n+func farewell() string { return \"bye\" }"},
             {"author": "dave", "body": "Done.", "at": "${d1}T07:30:00Z"}
           ]}
        ]
      }
    }
  }
}
JSON
}

mkdir -p "$dir/.remotes/demo" "$dir/.remotes/records/proposals" "$dir/.remotes/records/adr" "$dir/.remotes/records/notes"

cat >"$dir/.remotes/demo/README.md" <<'EOF'
# demo

A tiny Go program for trying ws missions. Run it with `go run .`.
EOF
cat >"$dir/.remotes/demo/go.mod" <<'EOF'
module example.com/demo

go 1.22
EOF
cat >"$dir/.remotes/demo/main.go" <<'EOF'
package main

import "fmt"

func greeting() string { return "hello" }

func main() { fmt.Println(greeting()) }
EOF
cat >"$dir/.remotes/demo/main_test.go" <<'EOF'
package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting(); got != "hello" {
		t.Errorf("greeting() = %q", got)
	}
}
EOF
make_remote demo
seed_landed_demo
seed_pull_requests

cat >"$dir/.remotes/records/README.md" <<'EOF'
# records

Proposals, ADRs and notes, shared through main.

- `proposals/<short-name>.md`: a title, the problem, the proposal, and what changes.
- `adr/NNNN-<short-name>.md`: numbered from 0001. Context, decision, consequences.
- `notes/<short-name>.md`: anything worth keeping that isn't a decision.
EOF
touch "$dir/.remotes/records/proposals/.keep" "$dir/.remotes/records/adr/.keep" "$dir/.remotes/records/notes/.keep"
make_remote records
seed_landed_records
seed_reading_records
# records takes pushes to main, as a hosted remote does (ws land pushes
# there): its files here follow.
git -C "$dir/.remotes/records" config receive.denyCurrentBranch updateInstead

# The workspace root is the config repo, as `ws init` would clone it.
cat >"$dir/ws.toml" <<'EOF'
[workspace]
org = "example-org"
default_branch = "main"
display_name = "ws test"

[repos.demo]
display_name = "Demo"
aliases = ["dm"]

[repos.records]
display_name = "Records"
aliases = ["rec"]
landing = "direct"

[missions]
records = "records"
# Guarantees: what ws holds on every mission, whatever an agent reads.
# The process says the same in prose, for the planner and the
# orchestrator.
review_before_pr = true

# Artifacts (versioned HTML prototypes) are on this machine. One you
# promote lives in records too, for everyone with it: the version chosen,
# in a folder there, a commit landed on its main each time it's promoted.
[artifacts]
path = "records:artifacts/{date}-{slug}/"

# records' kinds of document: where each goes, its commits, and how one's
# written (the records' README says).
[documents.adr]
title = "ADR"
path = "records:adr/{number}-{slug}.md"
add = "Add ADR {number}: {slug}"
amend = "Amend ADR {number}: {slug}"
guide = "records:README.md"

[documents.proposal]
title = "Proposal"
path = "records:proposals/{slug}.md"
add = "Propose {title}"
amend = "Amend the proposal {slug}"
guide = "records:README.md"

[documents.note]
title = "Note"
path = "records:notes/{slug}.md"
add = "Add a note: {title}"
amend = "Amend the note {slug}"
EOF
# How work goes here, in prose: the planner and the orchestrator read it as
# it's written, and the Settings page shows it.
cat >"$dir/PROCESS.md" <<'EOF'
# How we work

Anything more than a fix to demo is proposed first: a proposal in records,
which the team reads, and once it's accepted it can be built. We always
review code ourselves before a pull request is opened.

Bugs in demo are reproduced first, with a failing test, then fixed, and a
pull request opened. A bug's mission stays open until the fix is deployed.

Decisions worth keeping are written up as ADRs in records.
EOF
printf 'repos/\nws.local.toml\n' >"$dir/.gitignore"
git_seed -C "$dir" init -q --initial-branch=main
git_seed -C "$dir" add ws.toml PROCESS.md .gitignore
git_seed -C "$dir" commit -q -m "ws test config"
printf '/.remotes/\n/.ws/\n' >>"$dir/.git/info/exclude"

# Each repo the way ws lays it out: a bare clone and its .ground worktree.
for name in demo records; do
	repo="$dir/repos/$name"
	mkdir -p "$repo"
	git clone -q --bare "$dir/.remotes/$name" "$repo/.bare"
	git -C "$repo/.bare" config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'
	git -C "$repo/.bare" fetch -q --all
	git -C "$repo/.bare" worktree add -q "$repo/.ground" main
done

# Per machine: Claude Code on haiku in every role. Launch crew's menu
# offers crew on sonnet too, to try picking one. Every repo here is new to
# Claude, so its first crew would open on Claude's trust dialog, with
# nobody there to answer it when an agent's driving: trust_folders has ws
# answer it.
cat >"$dir/ws.local.toml" <<'EOF'
[crew]
harness = "claude"
model = "haiku"
allowed = ["claude:sonnet"]
trust_folders = true

[briefing]
harness = "claude"
model = "haiku"

[orchestrator]
harness = "claude"
model = "haiku"

[conversation]
harness = "claude"
model = "haiku"

[judge]
harness = "claude"
model = "haiku"

# GitHub isn't real here: this file answers for it (pull requests, stacks,
# review threads), and remembers the reviews you submit.
[github]
fixture = ".remotes/github.json"

# Passes over a review (the web UI's Reviews page, a pull request's
# Passes view): an agent reads the change and says what it finds, each finding
# on its lines, to take into your review or dismiss. "check" is given the
# change and nothing else; "tests" reads the code, so ws docks the pull
# request for it.
[reviews.passes.check]
title = "Check"
about = "What's wrong, unclear, or would trip a reader."
looks = "Read the change as its reviewer would. Find what's wrong (a bug, a case it misses, something it breaks), what's unclear, and what the description says that the change doesn't do. Don't remark on style a formatter would fix."
cold = true

[reviews.passes.tests]
title = "Tests"
about = "Whether what it changes is tested."
looks = "Find behaviour the change adds or alters that no test covers (untested), and tests that would pass whether or not the code is right (weak). Read the tests that are there before saying one is missing."
kinds = ["untested", "weak"]
blocks = ["untested"]
EOF

cat <<EOF
Test workspace ready: $dir

  cd $dir
  ws web               # Conversations: say what you want, and its agent proposes a mission
  ws process           # PROCESS.md, as the planner and the orchestrator read it

Claude Code (haiku) plays every role (ws.local.toml), and ws answers its
trust dialog for this workspace's repos ([crew] trust_folders). GitHub isn't
real here (org example-org): .remotes/github.json answers for it, with open
pull requests on demo for the web UI's Reviews page (ws web /reviews), and
a few days of merges and commits for its Team page (ws team). A review you
submit there is written into that file, not posted anywhere.
Remove the folder and run this again for a fresh one.
EOF
