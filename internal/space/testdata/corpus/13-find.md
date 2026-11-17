# Find

Status: the first two phases are built on `v2`, 2026-11-10, and tried in Chrome on a throwaway workspace: what's typed read as a ref, pull requests, files from the server's index with what each capsule has changed, groups, narrowing, and the card beside the list. See [What's built](#whats-built) and [What building it taught](#what-building-it-taught). The rest is a proposal. It's about the web UI's find window (`⌘K`, `web/src/shell/Palette.tsx`), and leans on [refs](11-refs.md) for what a thing is called and how it's drawn.

Find is one box that gets you to anything the workspace has, by whatever you remember of it: a file's name, a pull request's number, a few words of a decision, an address copied from GitHub.

## Where it stood

Before this, Find looked through what the page already holds of the workspace, and nothing else.

| Found | Didn't |
|---|---|
| The six places (Missions, Reviews, Files…) | A file, in any repo or capsule |
| A mission, by its title or slug | A pull request, by number, title, author or branch |
| A mission's capsules and crew | A review, a document, a decision, a thread |
| A page or a conversation, by its title | What a page or a conversation says |
| A headline's action (*Accept*, *Archive*) | Anything else you can do: a page's own keys, what the deck does |
| The theme, the keys | A commit, a symbol, text in the code |

- **Reviews is a place, not something to look in.** The workspace says how many pull requests need you, and Find knows no more.
- **It can't be opened from a page.** In a document being edited `⌘K` is a link, so Find is only the button in the sidebar there.
- **Empty, it's a fixed list:** the first forty things in the order they were added.
- **There are three matchers.** Find has its own (`score`), the file finder in Files has `ui/fuzzy.ts`, and the one in a review has a third, inside `Review.tsx`.
- **An address means nothing to it.** `demo#12` typed, or a GitHub link pasted, matches nothing.

## What it should find

| Kind | By | From | Goes to |
|---|---|---|---|
| **A ref, exactly** | Anything [refs](11-refs.md#kinds) reads: `demo/main.go:12-20`, `demo#12`, `fix-token-refresh/d2`, a GitHub address pasted | The resolver | The thing |
| **A file** | Its path, fuzzily, with the file's name counting most; `:12` goes to a line | Each repo's tree, and its capsules' | Files |
| **A pull request** | Its number (`12`, `#12`), title, author, branch | The list Reviews keeps | Its review |
| **A stack** | Its title, any layer's number | The same | Its bottom layer |
| **A review of a capsule** | The capsule, where it stands | Missions | The review |
| **A document** | Its title, its path | Missions' artifacts | The document |
| **A decision** | What was decided, and why | Missions | It, in its mission |
| **A thread** | The words it's on, its first message | Missions' conversations | It, on its document |
| **A mission** | Title, slug; archived ones too, below the rest | The workspace | The mission |
| **A page** | Its title, its headings, what its cells were asked | Pages | The page, at that place |
| **A conversation** | Its title, its summary | The workspace | It |
| **Crew, an orchestrator's run** | Id, mission | The workspace | It |
| **A commit** | The start of its id | Each repo | The pull request it's in, or GitHub |
| **Something to do** | Its name | See [Doing](#doing) | It's done |
| **Text** | Words, in the code or in what ws keeps | See [Text](#text) | The line |

- **A ref comes first.** What's typed is read as a ref before anything else. If it names something, that's the top line, whatever else matches. Paste a pull request's address from GitHub and `⏎` opens it here.
- **A number alone is a pull request.** `812` lists `demo#812` and `api#812`. One that isn't in the kept list (merged long ago) is offered as *Open demo#812*, and GitHub is asked only when you choose it.
- **A file's line.** `main.go:12` finds the file and goes to the line.

## The window

**Empty,** it's where you'd likely go:

- **Needs you:** the few things waiting, as the sidebar counts them.
- **Recent:** what you last opened, of any kind, newest first.
- **Places.**

**Typing,** results come in groups:

```
 ⌕ token ref

 Best
   ⚑ Fix token refresh                          ready        Mission

 Pull requests
   ⑂ demo#12   Refresh the token before it expires   merged
   ⑂ api#89    Token refresh: retry on 401           open · ✓

 Files
   ▤ demo/internal/token/refresh.go
   ▤ demo/internal/token/refresh_test.go
   3 more                                                      ⇥

 Decisions
   ◆ d2  Refresh on a timer, not on a 401       fix-token-refresh
```

- **Best** is the one thing that fits most: a ref typed exactly, or the highest of any kind.
- **Then a group for each kind,** a few of each, in an order that doesn't change as you type.
- **`⇥` narrows** to the group the cursor's in, and shows all of it. `⇥` again goes back.
- **The letters that matched are marked.**
- **What's on the right is where it stands:** a pull request's state and checks, a mission's headline. The same icons and words as a [tag](11-refs.md#drawn).
- **Beside it, on a wide window, the card.** The line the cursor's on shows what a ref's card shows: a file's first lines, a pull request's title and size, a decision and why. It's the same card, not a second one.

### Narrowing

A first character says what you're after. Each is also a chip under the box, to click.

| Typed | Only |
|---|---|
| `>` | Things to do |
| `#` | Pull requests and stacks |
| `/` | Files. `demo/` is files in demo. |
| `"` | Text |

Anything with a `/` or a `.go` in it leans to files without being told.

### Order

1. A ref, exactly.
2. What you've chosen before for these letters, and what you opened lately.
3. What needs you.
4. How well it matches: letters together, at the start of a word or a path's part, in a file's name over its folders.
5. Its kind: things with names (a mission, a pull request) over files, files over text.

What you chose before is kept in the browser, for each workspace.

### Keys

| | |
|---|---|
| `⏎` | Go |
| `⌘⏎` | Go, in a new tab |
| `⇥` | Narrow to this group |
| `→` | What else can be done with it |
| `⌘C` | Copy its name, as a ref |

`→` on a pull request offers *Review*, *Ask about it*, *Open here*, *Open on GitHub*. On a file: *Open in Files*, *Copy its path*. On a mission: what its headline offers, and *Park*, *Archive*.

### From anywhere

- **`⌘K` in a document** is Find when nothing's selected, and a link when something is.
- **`⌘P` is Find, narrowed to files,** anywhere. In Files and in a review it's narrowed further, to that capsule or that diff, as `t` is today. Those two finders become this one, given what to look in.
- **`@` in a page is Find, picking.** [Refs](11-refs.md#getting-into-the-file) has `@` open a list of what there is to name. It's this list. Choosing writes the ref into the page.

## Doing

Find already does a headline's action. There's more it could do, and the web UI has things the deck does that it can't yet ([audit](audit.md#1-the-web-ui-does-what-the-deck-does)): Find is the cheapest place to put them.

- **What this page's keys do.** Each page lists its keys as a key and a sentence (`missionKeys`, `pageKeys`), for the help. Made a key, a sentence and what it runs, they're things Find can do, with the key shown beside each. One list for the help, the keys and Find.
- **About where you are, first.** On a mission: *Accept the stage*, *Park*, *Dispatch*, *Record a decision*. On a page: *Run Check*, *Link what's named*. On a pull request: *Open here*, *Ask about it*.
- **Starting things:** a page, a conversation, a mission, a capsule (`ws lift`), a pull request opened here (`ws dock`).
- **What needs a word takes it in the box.** *Record a decision* then asks *What was decided?* in the same box, and `⏎` records it.

## Text

Looking in what things say is slower than looking at their names, and its results want more room. So it's asked for (`"`, or the *Text* chip), and never mixed into the rest.

- **In the code:** `git grep` on each repo's default branch, a few lines from each repo, with the line shown. `⏎` goes to it in Files.
- **In what ws keeps:** briefs, decisions, handoffs, pages, conversations' turns, threads. They're small files, read as they are.
- **It starts when you stop typing,** and what's still running is dropped when you type again.

## How it works

- **What the page holds is looked through in the page,** at once: missions, capsules, crew, pages, conversations, places, things to do. That's today, with one matcher.
- **The rest is asked of the server:** `GET /api/find?q=…&kinds=…`. It answers a kind at a time, so files don't wait on decisions. A new letter drops what the last one asked.
- **Each result is a ref,** with a title, a hint, where it stands and the letters that matched. The page draws it with what it already has for refs: the icon, the state, the card, where it goes.
- **What the server looks through is [the index](#the-index):** held in memory, built when first asked for, and never written to disk.
- **One matcher** (`ui/fuzzy.ts`, and the same in Go for what the server looks through), tested against one file of cases, as [the ref grammar](11-refs.md#how-it-works) is.
- **`ws find <words>`** is the same asked from a terminal, and `-f llm` for an agent. The graph lets an agent follow links from a ref, but not get to a ref from a few words: *which mission was about the token cache?* is a `grep` today.

## The index

The server keeps it, in `internal/find`. The server is the one process that outlives a tab, has git beside it, and is shared by every tab, the terminal and agents. The browser keeps only what it already has (the workspace's names) and what you opened lately.

Three rules:

- **In memory, and nothing on disk.** All of it can be read again from git and `.ws` in under a second. An index kept on disk is one more thing to be wrong.
- **Each part knows what it was built from,** by something cheap to look at: a commit, or a file's size and time. That's how [a cell's sources](10-pages.md#the-second-slice) are checked. Nothing is read again unless that moved.
- **It only looks while someone's looking,** as the server's event hub does. Opening Find warms it. Closed, nothing is checked.

### What's held

| Part | Holds | Built from | Stale when | Built in |
|---|---|---|---|---|
| **A repo's files** | Every path on the default branch, once | `git ls-tree -r` on ground's head | Ground's head moves. Git is asked for it at most every two seconds, and only while Find is asking. | Tens of milliseconds |
| **A capsule's own** | Only the paths it has changed, added or removed | The working tree against where the capsule branched (`git diff --name-status`), and what's untracked (`git ls-files --others`) | Older than three seconds, while Find is asking | Three git calls a capsule |
| **Pull requests** | The list `internal/pulls` keeps | `.ws/prs/list.json`, there once Reviews has been opened | Reviews reads it again | Nothing: it's already held |
| **What ws keeps** | A line for each decision, document, thread, page heading and cell's ask: its ref, its words, its mission | The files under `.ws` | A file's size or time moves | A few milliseconds |
| **Commits** | Nothing | `git rev-parse` in each repo, when what's typed is seven or more letters of an id | | Asked each time |
| **Text in the code** | Nothing | `git grep` on ground | | Asked each time |
| **Text in what ws keeps** | The words of the lines above, and of pages and turns | The same files | The same | Read with them |

### A capsule holds only what it changed

A capsule is the default branch with a few files different. Listing all of it would hold every repo once for each capsule, and Find would show `main.go` six times.

- **A file is one line, from ground.** Where capsules have changed it, the line says so (*changed in demo/token-cache*), and `→` goes to it there.
- **A file only a capsule has** (added, not merged) is its own line, named as the capsule's.
- **A file a capsule removed** is still ground's line.
- **Against where it branched, not ground as it is now.** A capsule behind the default branch would otherwise seem to have changed everything that's merged since.
- **Every capsule a repo has,** a mission's or not.
- **Its name is said of its files, and isn't letters they're found by.** Every file of a capsule called `cache` would otherwise be found by *cache*. With a colon typed (`token-cache:store`) it's the capsule that's asked for, and its name counts. The capsule is found by its name, as a capsule.
- **They're read when Find opens,** four at a time. An answer waits 150ms for capsules still being read, then goes without them and says so (*Reading 3 capsules*). A file crew made a moment ago is found a few seconds on.

This is also less work than today: Files reads a capsule's whole tree (to twenty thousand paths, `TreeMax`) each time it's opened.

### What ws keeps

Small files, many of them: a mission's `decisions.jsonl`, each conversation's `t<n>.jsonl`, the pages.

- **Each file is read once** and kept as the lines it gives, with its size and time.
- **The server already knows when to look.** It works out a fingerprint of the workspace every two seconds while a page is open. When that moves, the files' sizes and times are looked at, and only those that moved are read again.
- **No inverted index.** Looking through a few thousand short lines is under a millisecond. One is worth building only when that's measured and slow.

### Matching

- **Refused before it's scored.** Each path is held with a mask of the letters and digits in it. What's typed has one too, and a path missing any of its letters is skipped with one comparison. Most are.
- **The best few, not all sorted.** A kind keeps as many as were asked for as it goes, and a count of the rest.
- **A repo at a time, all at once.** Each repo's paths are looked through on their own, and the best of each put together.
- **Narrowing as you type.** With one more letter, what matches is among what matched before. If it's ever needed, the last answer's paths can be kept for the next. Not to start with.
- **Held once.** Paths are kept as written and compared without regard to case as they're read. Half a million paths is about thirty megabytes.
- **No cap.** `TreeMax` is for a tree drawn in the browser. The index holds every path.

### Asking

- **One request for each change to what's typed,** answered as a stream, a kind at a time as each is ready. Not one request a kind: a browser allows six connections to a host, and the web UI has run out of them before.
- **A new letter drops the last request.** The browser abandons it, the server sees that, and stops: a `git grep` still running is killed.
- **Opening Find sends an empty one,** which warms what's stale before the first letter.
- **`ws find`** asks the server when one's running. With none, it reads what it needs for that one question, and pays for it.

### How long it may take

| | From the key to the result | Measured |
|---|---|---|
| Names the page holds | The same frame | |
| A ref, pull requests, files | 50ms, with half a million paths | Files: 19ms for one letter, 4ms for three, 2ms for six, 0.2ms for letters nothing has (`BenchmarkFiles`, ten repos of fifty thousand paths, an M1) |
| What ws keeps | 100ms | Not built |
| Text | Starts after a quarter of a second's pause; first lines within a second; given up on at three, saying so | Not built |
| Opening Find, cold | Ground's trees within 200ms; capsules as they come | Not measured on a large repo |

The files' is a benchmark in `internal/find`, on paths made up for it. It times the looking, not git: reading a large repo's tree hasn't been timed.

## What's built

Phases 1 and 2: refs, pull requests and files.

- **The index** (`internal/find`):
  - `match.go` is the web UI's matcher in Go, with the mask that refuses a path before it's scored. `testdata/fuzzy.json` is the cases both are held to: scores and marked letters, filled in from the Go one (`go test -run TestFuzzyCases -update`) and checked by `web/src/ui/fuzzy.test.ts`.
  - `index.go` holds each ground's paths and each capsule's changes, reads them when they're due, and answers from a snapshot. A read is started once for a worktree whoever asks, and goes on if the request that started it is dropped.
  - `files.go` looks through them: a ground's file once with the capsules that changed it, a capsule's own as its own, lines (`:12`, `:12-20`) carried to every result, and with nothing typed, what capsules have changed.
  - `pulls.go` matches pull requests term by term: a number (`12`, `#12`, `demo#12`), who it's by (`sam`, `@sam`), its title, its branch.
  - `ref.go` reads what's typed as a name or an address: a file, its lines, a folder, a capsule's file, a path on disk, a pull request, and each by its GitHub address. It's the start of the grammar [refs](11-refs.md#how-it-works) asks for.
- **The API** (`internal/web/find.go`): `GET /api/find?q=…&kinds=…&limit=…` answers a line of JSON for each kind as it's ready (pull requests, the ref, capsules, files), then `{"done":true}`. With no `q` it warms the index. Pull requests come from what Reviews keeps (`Peek`), and GitHub is never asked.
- **The window** (`web/src/shell`):
  - `find.ts` is Find apart from how it's drawn: what the page holds as items, what's typed and its scope, what to ask the server, and `build`, which puts both into the sections shown, with the best line.
  - `recent.ts` is what it remembers, in the browser for each workspace: the last forty places you went (from Find, or any other way), and what you chose for what you typed.
  - `Palette.tsx` draws it: the box, the chips, groups, rows with matched letters marked and a path's folders quiet, the foot's keys, and the card (`RefCardBody`, from refs) beside the list on a window 1100px or wider.
- **Keys:** `⏎` goes, `⌘⏎` in a new tab, `⇥` narrows to the group the cursor's in and back, `⌘C` copies the row's name. `⌘K` opens it from a document when nothing's selected. `⌘P` opens it for files, except in Files and a mission's explorer, which have their own.
- **One matcher:** Find and a review's *Jump to a file* now use `ui/fuzzy.ts`. Each had its own.
- **Tests.**
  - `internal/find`: the shared cases; files of the default branch; a capsule holding only what it changed (changed, added, removed, renamed, untracked, ignored); a capsule behind; read again only when it's moved; a dropped request; pull requests by every form; twenty-one forms of ref and eighteen things that aren't one.
  - `internal/web`: `TestFindAPI`, the stream end to end with a real repo and pull request.
  - `web/src/shell/find.test.ts`: what's typed, grouping, the best line, narrowing, recents, a ref first, capsules named once.
- **In Chrome,** on a throwaway workspace (repos `demo` and `records`, a capsule with a file changed, one added and one untracked, and the stand-in for GitHub):
  - `main` put `demo/main.go` best, marked *changed in token-cache*, with its lines in the card. `maing:3` and `⏎` opened it in Files at line 3.
  - `store` found the capsule's own file as `demo/token-cache:internal/token/store.go`, and `⏎` opened it in the capsule.
  - A GitHub address of a file with `#L2-L4` was the best line, at lines 2 to 4. A pull request's address was the pull request.
  - `13` put `demo#13` best, with its place in its stack and *needs your review*. `⌘C` copied `demo#13`, and `⏎` opened its review.
  - Opened again with nothing typed, it listed those three under *Recent*.
  - `⇥` on a pull request made it `#gre`, and again `gre`.
  - `⌘K` in a page's editor opened Find, and with words selected, the editor's link.
  - Before Reviews had been opened, `#` said pull requests show once it has.
  - It read well at 390px wide, without the card, and in dark.

## What building it taught

- **The page's matcher threw away matches.** A start whose letters scored below nothing (far apart, none on a word's start) was taken for no match. Found writing it again in Go, and held by a case both are tested against.
- **A capsule's name can't be letters its files are found by.** The first cut matched `demo/token-cache:scratch.txt` whole, so *token* and *cache* found every file the capsule had. Its name is said of them now, and only counts after a colon.
- **A hint matched letter by letter finds too much.** *main* matched *Start a conversation* through "new **m**ission ide**a** quest**i**o**n**". A hint is matched by the start of its words.
- **`demo#1` is on its way to `demo#12`.** A pull request that isn't in hand was the best line as soon as its name was whole, in front of those that are. It's offered after them, unless there are none.
- **An answer that waits for nothing is a letter behind.** Capsules read again in the background were found on the next request, not this one. An answer waits a moment for them.
- **Between letters the list emptied.** What the last answer had is kept, to what still matches by the page's matcher, until this one's comes.
- **A path cut short from the right loses its name.** Read from the end, with its folders cut, it needed `<bdi>` round it, or its parts came out in the wrong order.
- **One letter over half a million paths took 70ms** in one go, and 19ms a repo at a time.
- **Formatting the new files with the formatter's defaults broke every object over several lines.** The project has no formatter set up, and its code is as wide as 200. Checked before anything else was touched.

## Not done

- **A folder named exactly goes to its repo's files,** not to the folder: Files can't open at one.
- **`→`, for what else can be done with a row.** Nothing's offered but going.
- **Find in Files and in a review is still each page's own,** not this one narrowed. They share the matcher.
- **A ground's head is asked of git,** not read from its ref file. It's a process a repo every two seconds while typing.
- **The card for a pull request says what Find was told,** and never asks GitHub for more.
- **Pull requests aren't there until Reviews has been opened once.** Find doesn't ask GitHub, so there's nothing to look through.
- **What's recent is kept for a pull request or a file as it was:** a pull request since merged is still listed.
- **Capsules of archived missions are read like any other,** while they're on disk.
- **A file's lines aren't checked:** `main.go:900` goes to a file of seven lines.

## Phases

Phases 1 and 2 are built.

1. **Refs and pull requests.** What's typed read as a ref, with an address pasted. Pull requests and stacks from the kept list. One matcher in place of three. Empty shows what needs you and what's recent. `⌘K` from a document.
2. **Files.** The server's find, trees kept, `:line`, `⌘P`, and the card beside the list.
3. **What ws knows.** Documents, decisions, threads, a page's headings and cells, archived missions, a commit by its id. Groups and `⇥`.
4. **Doing.** A page's keys as things to do, the deck's verbs, `→`.
5. **Text.**
6. **`ws find`,** and `@` in a page.

The first needs no new API beyond the resolver, and is most of what's missed day to day.

## Decided

Proposed, to agree:

- **One window,** for going, doing and picking. The file finders in Files and a review are it, narrowed.
- **A ref typed or pasted comes first.**
- **Results are refs,** drawn and previewed as refs are anywhere.
- **Names in the page, the rest on the server.** No tree in the browser.
- **The index is in memory, and nothing of it is on disk.**
- **A capsule holds only what it changed** since it branched. A file is listed once, from ground.
- **Text is asked for,** not mixed in.
- **Find doesn't call GitHub** until you choose something only GitHub has.

## Open questions

- **Groups or one list.** Groups keep files from burying a mission. One ranked list gets the best five to the top whatever they are. Groups with a *Best* line is a guess at both.
- **Does text belong here?** Results with lines of code want a page (`⌘⇧F`), with the files down the side. Find could hand over to it.
- **Pull requests GitHub has and the list doesn't,** by their words: closed ones, a teammate's from last year. Asking GitHub as you type is slow and counts against its limits.
- **Symbols.** Finding a function by name wants the index [refs](11-refs.md#open-questions) doesn't have either.
- **How fresh a capsule is.** Read when Find opens and every few seconds while it's open, a file crew just made can be missed for that long. Watching the folders would close the gap, and ws watches nothing today.
- **When `git grep` is too slow.** On a very large repo, text wants an index of its own (trigrams, by commit), and that one would be worth keeping on disk.
- **Reading a large repo hasn't been timed.** Looking through half a million paths has. `git ls-tree` on a repo that size, and the first open after the server starts, haven't.
- **What's recent, and whose.** Kept in the browser, it's lost on another machine, and an agent's `ws find` has none.
- **More than one workspace.** Find is in one. Is another workspace's mission ever worth finding from here?
