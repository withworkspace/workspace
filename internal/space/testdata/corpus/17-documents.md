# Documents

Status: a proposal, 2026-11-11. Nothing's built. What came of talking it through is under [Decided](#decided); the rest is what I'd choose, marked as a lean where it's a choice. It takes up [the library](12-workspace-standards.md#the-library-and-search) that workspace standards left as step 8, and changes how [pages](10-pages.md) are reached: *Pages* in the sidebar becomes *Documents*. It leans on [refs](11-refs.md) for citations and on [find](13-find.md) for its index.

Documents is where the workspace's writing is read. It's made of spaces: one for each repo that's documents rather than code (`cr-docs`, `cr-records`), and one called **Local** for pages that aren't in a repo. A space is a place to read and look around in, not a tree of files. Writing in one is what a page already is: `e` on any document edits it there, with cells and passes.

```
Documents
─────────────────────────────────────────────────────
Local                               3 pages · 1 filling
  Caching requirements          4 cells       2h
  Retry semantics scratch                      1d

Records  cr-records                  148 · 2 drafts
  Changed lately: accept bp portfolio-bundle (ana, 1d)
  ADRs 41   Build proposals 63 (12 proposed)   Notes 44

Docs  cr-docs                        69 · 1 in flight
  In flight: methodology/risk.md (edit-risk, yours)
  Domain · Methodology · Product · Technical design
```

## Why

- **Pages is a list of things you wrote here, and the documents are elsewhere.** Since workspace standards, a document is edited as a page. The list shows the pages, a few of them tied to files, and not the 220 or so documents in the two repos they'd be tied to. Reading one means Files, then the repo, then the folder, then *Open as a document*.
- **Files reads a library as a tree.** `cr-records` is about 150 documents named `2026-10-27-portfolio-bundle-contents.md` in `bp/proposed/`. The tree shows names, not titles; folders, not kinds; nothing of who changed what lately or what cites what.
- **A document has two addresses.** `/docs?ref=` reads it, and `/pages/:id` (by way of `/pages/open?ref=`) edits it. Links lead to the first, and what you were writing is at the second.
- **Files is for two kinds of repo at once.** The code repos you work in and the document repos you read sit side by side, so the 26-repo list holds two that are opened for a different reason, and opened differently, from the rest.
- **A page has no place but the list.** A page started as a build proposal, not yet filed, is in Pages beside a scratch page about retries. It belongs with the proposals.

## Spaces

A space is a repo, or a folder of one, that's read as documents.

- **Declared on the repo.** `documents = true` under `[repos.cr-docs]` in `ws.toml`, or at the top of the repo's own `ws.repo.toml`, which the workspace overrides (as `landing` is). Declared in the repo, it holds for every workspace that has it.
- **A folder of a code repo:** `documents = "docs/"`. Its markdown under that folder is a space, and the rest of the repo is code as before.
- **The records repo is one without saying.** `[missions] records` makes its repo a space, with `documents = false` to say not.
- **Yours to turn off.** `ws.local.toml` can say `documents = false` for a repo whose tooling you work on, and it's back in Files as code.
- **Not every markdown file.** Code repos' READMEs, changelogs and package docs stay where they are. Any markdown file still opens as a document (`/docs?ref=` as now); a space is what's browsable, and where a repo's files open.
- **Its name and colour are the repo's label,** as refs draw a repo. A space for a folder is named for its repo and folder (`api docs`).

### Local

Local holds every page whose document isn't in a space: pages with no file, and pages tied to a file in a repo that isn't one (`demo/README.md`, edited as a page, says so on its row). It's what Pages lists now, less what moves to spaces.

The name says the one thing that matters: it's on this machine and no one else sees it. *File as…* is the way out, and filing a page moves it to the space it's filed in. A repo can't be called `local` and be a space (`ws doctor` says so), since it would share the address.

## A page shows where it will live

The rule that decides where a page is listed:

| The page is | It's shown |
|---|---|
| Tied to a file in a space, with something to save | On that document, in its space, as your draft |
| Started as a kind whose path is in a space, not filed yet | In that kind's list, as a draft only you see |
| Filed | As the document it was filed as |
| Anything else | In Local |

- **A page with nothing to save** (a file opened as a page and not changed) isn't a draft, and isn't shown as one.
- **Each space says how many drafts of yours it has,** and lists them first.
- **A draft can be thrown away** from where it's shown: the page goes, and its cells' conversations stay in `.ws/conversations`, as a removed page's do now.

## A space's shape

A space is drawn from what it has. Every space opens on the same three things: its front page, what changed lately, and what's in flight.

### With kinds: a library

A space that the workspace's kinds point into is listed by kind, as [the library](12-workspace-standards.md#the-library-and-search) has it:

- **A list per kind:** its number or date, its title (its first heading), where it stands, and who changed it last and when. Sorted and filtered by each.
- **Where it stands is read from its folder.** *Proposed* and *accepted* are the folders a build proposal can be in. That needs a kind's moves to be read (not done: only where each goes). Without that, a file in `bp/accepted/` doesn't fit `bp/proposed/{date}-{slug}.md`, so it's of no kind. Reading `[documents.bp.moves.accept] to` is enough for the library to list it as a build proposal, accepted.
- **Everything else in the space,** by folder under *Other*: a README, the presentations, a guide.
- **New** (`n`) offers the space's kinds, and starts a page as one, as *New as…* does now.

### Without kinds: folders

`cr-docs` has 69 documents in four folders and no kinds.

- **Its folders are its sections,** and each document is listed by its title, not its file's name.
- **Its front page** is its README: a front page that links the rest is how the repo means to be read. Lean: no setting for another (`cr-docs`' `llm-index.md` is for agents) until one's wanted.
- **New** asks for a folder and a title, and starts a page tied to where it'll go. Filing to a plain path, with no kind, is new: *File as…* takes a kind today. It lands as the repo does: a capsule and a pull request for `cr-docs`.

### Changed lately, and in flight

The top of a space, so it reads as somewhere people are working:

- **Changed lately:** the space's last commits on its default branch, by who, what they said, and which documents (by title). From the ground's log, so as fresh as the ground (see [Fresh](#fresh)).
- **In flight:** capsules of the space's repo with changes to its documents, yours and others', and the pull requests open on them. A document in flight says so on its row, and in its page.
- Lean: capsules first, which ws has already; pull requests by their branch matching a capsule, and the files a pull request touches after.

## A document is one place

- **One address:** `/documents/cr-records/bp/proposed/2026-10-27-portfolio-bundle-contents.md`, and a local page's `/documents/local/p3`. `/docs?ref=`, `/pages/:id` and `/pages/open?ref=` lead there.
- **Read first, edited in place.** It opens as it's rendered. `e` edits it there: the page tied to the file, made the first time, with its cells, questions, *Change*, the map and its kind's passes. The address stays, with `?edit`, so a reload keeps you writing. *Save to repo* and *Bring in what changed* are as they are now.
- **Its version in a capsule** is read at the same address with `?in=edit-risk`, from *In flight*.
- **Beside it,** in the margin when it's read:
  - your draft of it, if you have one, and how far it's moved from the file;
  - in flight: a capsule or a pull request that changes it;
  - **cited by:** the documents in any space whose refs name it (the build proposals that cite an ADR, the ADRs a proposal spawned), from [the store's document cache](19-store.md#what-stays-outside) (`doc_links`);
  - its kind, and its moves (*Accept*) once they're built.
- **A mission's document stays the mission's.** A space shows what's on the default branch. A file a mission records opens as the mission's document, as it does now, and a space's row for it says whose it is.

## Files gives them up

A repo has one home in the web UI. A code repo's is Files. A space's is Documents, and every way to one of its documents leads there.

- **In Files, spaces fold away.** Whole-repo spaces go to a group at the foot of the repo list, *In Documents*, closed, as a hidden group is now. Its head links to Documents.
- **Folded, not removed.** A space still has files that aren't documents (images, the presentations, `.vale.ini`, the skills in `.agents/`), and capsules whose files are worth looking at as files. Opened from the fold, a space's repo is Files as before, and ⏎ on a markdown file in it opens it in Documents.
- **Every way in leads to Documents:** Find (`⌘K`), refs (`cr-records:bp/…`, and citations such as `ADR-0038` once refs read them), links between files in a document, and the Team page's lists of what changed. A file in a space that isn't markdown opens in Files.
- **A folder space** leaves its repo in Files whole. Only links to markdown under its folder lead to Documents.
- **It isn't a hidden group.** A hidden group is left out of commands that cover every repo. A space is fetched, listed and checked like any repo. It needs a flag of its own, and gets one (`documents`).

## Fresh

A space reads its ground: the default branch, as last pulled. A place to read that's a day behind is worse than Files being behind, because you'd take it as current.

- **Fetched when it's looked at.** Opening a space fetches its repo if it hasn't been for a few minutes, as the Team page fetches while it's open, and not otherwise.
- **Said when it's behind.** A space whose ground is behind its remote says how far, and pulls it (a fast-forward of a ground with nothing in it) when you say.
- Lean: pulling without asking. A ground is read-only by rule, so a fast-forward can't lose anything. It's asked here only because nothing in ws moves a ground by itself yet.
- **Titles, sections and refs are read once for each document's content,** into [the store's document cache](19-store.md#what-stays-outside), not per request. When a ground's head moves, only the documents a commit changed are read again.

## Order

Each slice is useful alone.

1. **Documents, in place of Pages.** The sidebar, Local and the spaces from config, each space a list of titles by folder, read from [the store's document cache](19-store.md#what-stays-outside), which comes first. Documents open in the reader as now. Pages are listed by [the rule](#a-page-shows-where-it-will-live). Spaces fold away in Files.
2. **One address.** `/documents/…`, read and edited in place; the old addresses lead there. Find, refs and links between files open documents in their space.
3. **Libraries.** Lists by kind, with where each stands, which needs a kind's moves read. *New* from inside a space, and filing to a plain path in a space without kinds.
4. **Changed lately, in flight, and fresh.**
5. **Cited by, and search** over the spaces, from a box at the top of Documents (`/`).
   - **Search is ranked.** `ws rg` over 220 documents gives lines in the order of their files. For prose, what's wanted is the ADR about the thing: by its title first, then by the section that says it. So documents are read by section into a table that ranks ([the store](19-store.md#what-stays-outside), `doc_words`), kept by each document's content, so only what a commit changed is read again. `ws rg`, scoped to a space, stays for an exact string.
   - **Ground only.** A document in flight in a capsule isn't searched; its row says it's in flight.
   - **A group of its own in Find,** beside what ws keeps, not mixed into it.
   - **By kind and where it stands,** once slice 3 reads them: *accepted build proposals about X*.
   - **Cited by** is from the same reading (`doc_links`).

## Decided

- **Pages becomes Documents,** made of spaces: one per repo that's documents, and Local for pages not in a repo (2026-11-11).
- **A space's repo has Documents as its home,** and gives up its place among Files' repos (2026-11-11).

## Open questions

- **Kinds outside a space.** A kind whose path is in a code repo (`api:docs/adr/{number}-{slug}.md`) with no space there. Lean: `ws doctor` suggests declaring the kind's folder a space, and until then its documents open as any markdown file does. Or a kind is enough to make its folder one.
- **What you've read.** Kept on this machine, "read last week" and "changed since you read it" would make a space feel like your library. Worth it, or noise beside *Changed lately*?
- **Reviews of a space's pull requests.** A pull request on `cr-docs` is reviewed as code, line by line. One that only changes documents could open as the document, with what changed tracked in it.
- **Presentations and other files a space has** (front matter for a slide tool). Listed under *Other* and opened in Files, or drawn as they're meant to be?
- **A space across repos.** Two records repos, or records and a folder of docs, read as one place. Lean: not until someone has the case.
