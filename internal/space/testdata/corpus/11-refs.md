# Refs

Status: a proposal, 2026-11-10; the first of it built on `v2`, 2026-11-11: one way of naming, said to every agent, and one finder, which every reader draws (see [Phases](#phases)). The rest is still to build. How GitHub draws each written form was checked against its own renderer (see [Checked](#checked)). It takes up what [pages](10-pages.md#refs) left not done about refs, and changes one thing pages decided: that a ref is only its words. It builds on the [information graph](01-information-graph.md), whose refs these are.

A ref is a name for something, written in a document: a file, some lines of it, a pull request, a mission. This design does three things:

- **More kinds:** a directory, a symbol, a commit, an issue, a capsule, a decision, a pasted GitHub address.
- **Drawn as what they are:** a tag with an icon and where the thing stands, not monospace words or a long blue address.
- **Links in the file,** so a page reads and links on GitHub, in a pull request's description, or anywhere else markdown is read.

```markdown
The greeting is built in [demo/main.go:12-20](https://github.com/acme/demo/blob/4f2a9c1/main.go#L12-L20),
last changed by [demo#12](https://github.com/acme/demo/pull/12) for [fix-token-refresh](ws:fix-token-refresh).
```

| Read by | It reads as |
|---|---|
| ws | Three tags: the file's lines, the pull request with *merged* after it, the mission with *ready* after it. |
| GitHub, as a file or in a pull request | Two links, and *fix-token-refresh* as plain words. |
| `cat`, or an agent | Each name in brackets, with where it is after it. |

## Why

- **Six kinds aren't enough.** A page can't name a directory, a function, a commit or an issue, and a GitHub address pasted in is a line of blue.
- **Outside ws a ref is only words.** `demo/main.go:12-20` in a page kept in the records repo, or in a brief copied into a pull request, goes nowhere.
- **Lines drift.** `demo/main.go:12-20` means lines 12 to 20 of whatever the default branch holds today, which may not be what was meant when it was written.
- **A ref can't be refused.** Words are looked through every time they change, so a mission called `cache` is a ref wherever the word is, and there's no way to say it isn't.
- **Finding costs a tree.** To know `demo/nope.go` isn't a file, the page reads every path in the repo.
- **There are three finders.** `pages/refs.ts` for pages, `ui/hover.tsx` for `c1` and `d2` in a mission's views, and `ws show`'s shapes in Go. The reader (`Markdown.tsx`) draws none, so a pass's tab and a cell's panel have no refs.

## A ref is a link

Three things, kept apart:

| | |
|---|---|
| **What you write** | Its name (`demo/main.go:12-20`), or its address pasted from GitHub, or a pick from a list. |
| **What's in the file** | A markdown link. Its words are the name. Its address is where the thing is outside ws. |
| **What ws draws** | A tag, with a card on hover and somewhere to go in ws. |

- **The address says what it is.** ws reads a ref from the link's address, never from its words. So the words can be short (`main.go:12-20`) or your own (*the retry loop*) and it's still the same ref.
- **The words are for everyone else.** GitHub shows them as the link. An agent with the workspace on disk reads `demo/main.go:12-20` and opens the file, without going to GitHub.
- **Things only ws has get a `ws:` address.** A mission, a page, a decision. GitHub drops an address it doesn't know and leaves the words, so they read as text there, not as a link that fails.

### What was weighed

| Form | In ws | On GitHub | Read raw | |
|---|---|---|---|---|
| Words only (today) | Found and marked | Plain words | Cleanest | Goes nowhere outside, and can't be refused. |
| A bare address | Could be a tag | In a comment, a pull request's is drawn as `#12` with a card. In a file, the whole address. A file's is a long line everywhere. | Worst | Unreadable in the file, and no words for an agent. |
| `[demo/main.go]`, with its address at the foot of the file | A tag | A link | Clean | A paragraph copied out loses its addresses, and a cell would have to write outside itself. |
| HTML with `data-` attributes | A tag | The attributes are stripped | Poor | Nothing gained over a link. |
| **A link: name, then address** | A tag | A link | Fair | Chosen. Each ref carries what it needs, wherever it's pasted. |

## Kinds

*Name* is what's typed, and the link's words. *Address* is the link's. `acme` is the workspace's `org`.

| Kind | Name | Address | Card | Goes to |
|---|---|---|---|---|
| File | `demo/main.go` | `github.com/acme/demo/blob/main/main.go` | Its first lines | Files |
| Lines | `demo/main.go:12-20` | `…/blob/4f2a9c1…/main.go#L12-L20` | Those lines, and whether they've changed since | Files, at the lines |
| Directory | `demo/internal/` | `…/tree/main/internal` | What's in it | Files, at the folder |
| Symbol | `demo/main.go#greet` | The lines of its definition, as for lines | The definition | Files, at it |
| Commit | `demo@4f2a9c1` | `…/commit/4f2a9c1…` | Its subject, who and when, the files it changed | The pull request it's in, or GitHub |
| Pull request | `demo#12` | `…/pull/12` | Title, who it's by, where it stands, checks, size | Its review |
| Issue | `demo#12` | `…/issues/12` | Title, where it stands, who has it | GitHub |
| Capsule | `demo/token-cache` | `…/tree/token-cache`, once pushed | Its mission, commits ahead, its pull request | Files, in the capsule |
| A capsule's file | `demo/token-cache:internal/cache.go` | `…/blob/token-cache/internal/cache.go`, once pushed | Its lines, as they are on disk | Files, in the capsule |
| Mission | `fix-token-refresh` | `ws:fix-token-refresh` | Title and headline | The mission |
| Page, conversation | `p3`, `w4` | `ws:p3` | Title, and what's open or its summary | It |
| A mission's things | `fix-token-refresh/d2`, `/s1`, `/c1`, `/t3` | `ws:fix-token-refresh/d2` | What `ws show` says first | Where it shows in the mission |
| Elsewhere on GitHub | A pasted address | As pasted | What GitHub says of it | GitHub |
| The workspace's own | `ENG-482` | From `ws.toml` | None | The address |

- **A `ws:` address is what `ws show` takes.** `ws:fix-token-refresh/d2` is `ws show d2 -m fix-token-refresh`. `ws show` should take either kind of address whole, so an agent can pass on what it reads.
- **A capsule's file is named as the graph names it** (`repo/capsule:path`), not by its path on disk. `repos/demo/token-cache/internal/cache.go` is still read, since agents write it, and so is the whole path on this disk (`/Users/…/repos/demo/.ground/main.go`). After the colon there's a line or a path, so `demo/main.go:12` and `demo/token-cache:main.go` can't be taken for each other.
- **A repo's alias is read as the repo.** `api/routes.go` is `vertex-seclr/routes.go` where `api` is its alias. Agents are told to use the name, but write aliases anyway.
- **A pull request and an issue share a name.** GitHub numbers them together, and ws asks which it is.
- **Until a capsule's branch is pushed** it and its files have a `ws:` address. The tag says *not pushed*, and *Link again* gives it GitHub's once it's there.
- **A workspace's own** is a shape and an address: `[refs.linear] match = "ENG-(\\d+)"`, `address = "https://linear.app/acme/issue/ENG-$1"`. It's what GitHub's autolinks do, but written into the file, so it works wherever the file is read.

## Pinned

A link to lines is to a commit, not a branch.

- **Lines are of a commit:** the default branch's head as last fetched, when the link was made. That's what the lines meant, and it's on GitHub.
- **A file or a directory follows the branch.** Nothing about it is tied to a version.
- **Changed since.** ws compares the commit with what's there now. When the lines named have changed, the tag turns amber and its card shows both. *Link again* moves it to today's commit, with the lines carried through the change where they can be.

This is what [a cell's sources](10-pages.md#the-second-slice) do for a cell, done for your own words.

A capsule doesn't last: it lands, and is burned. A ref to a capsule's file is to its commit, as lines are, once the branch is pushed. Once the capsule has gone, its card says so (and the commit it was at), shows the file as it is on the default branch, where it most likely is now, with *On the default branch now*, and its tag goes there (built). Nothing's rewritten: the ref still says what was meant when it was written, and the card says where that is today.

## Getting into the file

Nothing is linked by hand.

| | |
|---|---|
| **Typed** | When a name is finished (a space, a full stop, `⏎`) and it names something that's there, it becomes a link. `⌘Z` takes back only the linking, and the words then stay words. |
| **Picked** | `@` opens a list of what there is to name: files, pull requests, missions, pages. Choosing one writes the link. Nothing with an `@` is written. |
| **Pasted** | A GitHub address becomes a link with the thing's name for its words. Pasted over a selection, the selection is its words. |
| **Written by an agent** | Agents write names, as they're told to now. The page links them as it writes the reply into the document, at the commit that was there when the agent read. An agent never needs the org or a commit. |
| **Already there** | *Link what's named*, for a document that came from elsewhere: it says how many it found, and links them. |

- **Linked once, not found each time.** That's what lets a ref be refused. `cache` taken back stays a word.
- **A name in backticks is linked with them** (``[`demo/main.go`](…)``), which GitHub draws as a link in code.
- **A name without its repo** (`main.go:5-7`) is linked when one file among the repos the page already names fits it. Its words stay as written.
- **Not in a code block or a heading,** as today.

## Found where it's said, linked where it's kept

A document is a file that travels, so its refs are links. A conversation's turn, a log line, a pass's finding and a cell's notes aren't files anyone else reads: names in them are found and drawn as they're shown, as today, and nothing is rewritten. Both go through one finder and draw the same tag.

What's found when it's shown is found against what's there then. A conversation read a month later names a capsule that's been burned, or lines that have moved. So a turn keeps, beside its words, what each name in it was when it was said (`Turn.Refs`, each a `ref.Pinned`: the ref, with the commit its lines or its capsule were at, the capsule's branch and whether it was pushed), and its tags are drawn from that first. The words aren't touched. Built 2026-11-11, for an agent's turns and what's said in a thread.

## Where it ends up

One grammar, one resolver, and a renderer for each place the words go. Agents, and you, write names. What ws does with them depends on who reads them next.

| Where | What's drawn | Rewritten? |
|---|---|---|
| The web UI's reader: a conversation, a thread, a pass, a cell's notes, a handoff, a log line | Names found as they're shown, drawn as tags with a card | No |
| A page or a document, in its file | `[name](address)`, written once, when the name is | Once, when it's written |
| Leaving ws: a pull request's description, the records repo, anything posted to GitHub | GitHub's addresses; what only ws has as its words | The copy that leaves |
| An agent, or a terminal (`ws show -f llm`) | The name, with where it is on this disk beside it | Added to, not changed |

A document read where it is (a file in a repo) follows its own relative links, but a link to one of the workspace's repos on GitHub (`github.com/<org>/cr-records/blob/main/adr/0026.md#decision`) is read as what it names and stays in ws: Files at that file and heading, a pull request's review. Nothing in the file changes.

The reader also finds what agents get wrong, so it can be drawn as what it means: a path on this disk, an alias, a link whose address is a path on this disk (`[auth config](/Users/…/appsettings.prod.json:15)`), and a name in backticks.

## Drawn

| Link | Drawn as |
|---|---|
| Its words are the thing's name, or its address | **A tag:** an icon for its kind, the name, and where it stands. |
| Its words are your own (*the retry loop*) | **Linked words:** the words as written, with the kind's icon before them and the same card. |
| Anything else | A link, as now. |

What a tag says:

| Kind | Name shown | After it |
|---|---|---|
| File, lines, symbol | `demo/main.go:12-20`; a long path with its middle left out (`demo/…/cache.go`) | *changed since*, *gone* |
| Directory | `demo/internal/` | |
| Commit | `4f2a9c1` and its subject, cut short | |
| Pull request, issue | `demo#12` | open, draft, merged or closed, and its checks as a dot |
| Capsule | `demo/token-cache` | *not pushed*, or its pull request's state |
| Mission | Its title | Its headline (*ready*, *asks*) |
| Page, conversation | Its title | What's open on it |
| A mission's thing | `d2` and its first words | Withdrawn, resolved |

- **It sits in a line.** A tag is the height of the text, wraps like a word, and doesn't change the line's spacing.
- **A tag is one thing to the cursor.** Arrow keys step over it, and backspace selects it before removing it. Linked words are edited as any words.
- **Click opens its card and keeps it open; `⌘`-click goes there.** Where the document is only read, a click goes. The card has *Open on GitHub*, *Copy name*, *Change the lines*, *Link again* and *Unlink*.
- **How each kind is drawn is said once** (`web/src/refs/views.tsx`, built 2026-11-11): a kind's `Look` is its shape (words, or a pill), its icon, an accent colour, text shown in place of its name, and what's said after it. The reader's tag, `Inline`'s and the page editor's marks all draw from it, and a kind's card is `RefCard`'s. A repo is a pill in its colour, by its label (`Demo` for `demo`). In the editor, text in place of a name shows while the cursor's elsewhere, and the name comes back to be edited when it's in it.
- **One tag, wherever a ref is shown:** the editor, the reader (a conversation, a pass's tab, a cell's panel), a log line, the map. `ui/hover.tsx`'s refs fold into it.

## Agents

- **Told once, the same everywhere.** `prompt.Naming` says how to name a file, a capsule's file, a pull request, a mission, a page and a conversation, and is in every prompt whose words the user reads: talk, threads, pages, crew and the orchestrator. Before it, only a page's agents were told, and a conversation's agent named a file by its path on disk, as a link to it.
- **Reading.** A prompt says once how to read them: a link to `github.com/acme/<repo>/blob/…` is that file here, and a `ws:` address is for `ws show`. The link's words already say which file.
- **Writing.** Names, not links. ws links them.
- **`ws show <address>`** takes a GitHub address of a workspace's repo, or a `ws:` one.

## How it works

- **One grammar** (`internal/ref`): a name or an address in, a ref out, and back. The browser has the same in TypeScript. Both are tested against one file of cases, so the two can't drift as the three finders have.
- **The server resolves** (`POST /api/refs/resolve`, many at a time): whether a file is there, a commit for the default branch, where a symbol is defined, whether `#12` is a pull request, whether a branch is pushed. The browser only tells what looks like a name, so a tree is never sent to it.
- **Where things stand comes with the workspace's events,** as a page's cells do: a tag changes when its pull request merges.
- **In the editor a ref is a link mark,** as any link is. The bridge doesn't change, and a block that wasn't touched is still written back byte for byte.
- **A symbol is found by its name in the file:** a line that defines it, by the language's shape (`func greet`, `def greet`, `class Greet`). The address is its lines at that commit. When they no longer define it, the tag says *moved* and *Link again* finds it.

## Phases

0. **Names everywhere** (built). One naming rule in every prompt. One finder (`web/src/refs`) with aliases, a capsule's file and the paths on disk, drawn by the markdown reader (`Markdown.tsx`: a conversation, a thread's replies, a pass, a cell's notes, a handoff, a brief) as the page's editor draws it, with the same card. `Inline` (a log line, a summary, a list row) draws them too, after a mission's own refs (`c1`, `d2`: `mission/refs.tsx`), as words that go there when clicked, since a line may sit inside a link. A card draws none (`NoRefs`). A repo by its name, and a folder, are refs. The finder makes its patterns once for what there is to name.
1. **The grammar and the resolver** (built, 2026-11-11). `internal/ref`, its TypeScript twin and their shared cases (`testdata/cases.json`): a name and an address in, a ref out, and back. find, a document's stored ref and a pull request's name are read by it. `ws show <address>` takes `ws:` and the workspace's GitHub addresses. `ops.RefsKnown` and `ops.Resolve` say what there is to name, and pin a name to now.
2. **Links drawn as tags.** The tag, linked words and the card, in the editor, the reader and `Inline`. Found words still draw as they do now.
3. **Written as links.** Typed, pasted, agents' replies, *Link what's named*, and taking one back. Lines pinned. Documents stop being looked through. *Built 2026-11-11:* a cell's reply has its names linked as it's written into the page (`ref.Link`, `ops.LinkNames`: lines at the commit the ground was read at, a capsule's file at its branch once pushed, else `ws:`); a page's *Link names* links what it names, in one step undo takes back; and a link whose address is a ref is drawn as the ref, in the editor and the reader, as its kind's `Look` says (words of the writer's own keep the icon and the words). *Not yet:* linking as you type or paste, *Unlink* and *Link again* on the card, and documents are still looked through for names that aren't linked.
4. **More kinds.** Directory, commit, issue, capsule, a capsule's file by its graph name, a mission's things.
5. **Changed since, symbols and `@`.**
6. **The workspace's own,** and repos elsewhere on GitHub.

## Checked

Sent to GitHub's markdown renderer (`gh api markdown`), once as a file and once as a comment in a repo:

- A link with a name for its words and a GitHub address is a link in both, with the words as written.
- A link with a `ws:` address is its words alone in both, with no link.
- A link around a name in backticks is a link in code.
- A bare pull request address is the whole address in a file. In a comment it's `#9000` with a card, and in a task list too. Inside a link with its own words, it gets neither.
- `org/repo#9000` and `org/repo@sha` are links in a comment and plain words in a file.
- `data-` attributes on a link are stripped.

Only GitHub was checked. What another reader does with a `ws:` address isn't known.

## Decided

Proposed, to agree:

- **In a document, a ref is a link.** Its words are its name, and its address is where it is outside ws. This replaces "a ref is its words" in [pages](10-pages.md#decided).
- **The address is what it is,** and the words can be anything.
- **What only ws has is addressed `ws:`,** so it reads as words elsewhere.
- **Lines are pinned to a commit.** Files and directories follow the branch.
- **Linked once,** when written, not found again each time.
- **ws writes the link.** You and agents write names.
- **Found where it's said, linked where it's kept.**

## Open questions

- **Crew and addresses.** An agent given a brief full of GitHub addresses may fetch them, not open the files it has. A cold start with and without them would tell.
- **A pull request's card on GitHub.** A linked `demo#12` in a pull request's description has no card or state, where a bare address would. *Copy for GitHub* could write those bare.
- **A mission renamed.** Briefing can change a slug, and `ws:fix-token-refresh` then names nothing. Missions have no id that stays.
- **Symbols without an index.** Finding by the shape of a definition will miss some (a method among several of one name, a generated file). A language server is the real answer, and a lot to carry.
- **A file moved.** A link to lines still works at its commit, but its words name a path that's gone. Does *Link again* change the words?
- **A snippet in a comment.** GitHub draws a bare address of lines at a commit as the lines themselves, in a comment. Worth writing bare when a ref sits on a line of its own?
- **Other hosts.** Addresses are GitHub's shapes. A repo on GitLab needs its own.
- **People.** `@name` is drawn by GitHub in a comment, and notifies them. Left out.
