# ws

A multi-repo workspace manager. `ws` gives development teams a single control plane for many repositories, using git worktrees to create lightweight, isolated working environments.

Work happens in **capsules**, not on the main branch. Ground is sacred.

## Table of Contents

- [Getting Started](#getting-started)
- [Concepts](#concepts)
  - [Workspaces](#workspaces)
  - [Ground](#ground)
  - [Capsules](#capsules)
  - [Lifting](#lifting)
  - [Docking](#docking)
  - [Boarding](#boarding)
  - [Debrief](#debrief)
  - [Landing](#landing)
  - [Repos and Aliases](#repos-and-aliases)
  - [Groups](#groups)
  - [Searching](#searching)
  - [Spaces of documents](#spaces-of-documents)
  - [Finding what's been said](#finding-whats-been-said)
  - [Missions](#missions)
- [Configuration](#configuration)
  - [ws.toml](#wstoml)
  - [ws.local.toml](#wslocaltoml)
  - [ws.repo.toml](#wsrepotoml)
- [Shell Integration](#shell-integration)
  - [The ws Wrapper](#the-ws-wrapper)
  - [Tab Completions](#tab-completions)
  - [Starship](#starship)
- [IDE Integration](#ide-integration)
  - [VS Code / Cursor](#vs-code--cursor)
  - [IntelliJ](#intellij)
  - [Opening Your Editor](#opening-your-editor)
- [Runs](#runs)
- [Silos](#silos)
  - [The Problem](#the-problem)
  - [How It Works](#how-it-works)
  - [Commands](#silo-commands)
  - [How the Daemon Keeps Them](#how-the-daemon-keeps-them)
  - [Hooks](#silo-hooks)
  - [ws.repo.toml Silo Config](#wsrepotoml-silo-config)
- [Customisation](#customisation)
  - [Repo Colors](#repo-colors)
  - [Post-Create Hooks](#post-create-hooks)
  - [Fuzzy Matching](#fuzzy-matching)
  - [JSON Output](#json-output)
- [Plugins](#plugins)

---

## Getting Started

### Prerequisites

- Git
- [GitHub CLI](https://cli.github.com/) (`gh`) — used for PR integration and authentication.

### Creating a Workspace

A workspace lives in a shared config repo that contains a `ws.toml` file describing your repos. To initialise:

```bash
ws init git@github.com:your-org/workspace-config.git
```

This clones the config repo, then clones every repo listed in `ws.toml` as a bare repository with a `.ground` worktree for the default branch. The directory structure looks like this:

```
my-workspace/
  ws.toml
  ws.local.toml
  repos/
    frontend/
      .bare/
      .ground/
    backend/
      .bare/
      .ground/
```

If you prefer SSH for git operations:

```bash
ws init git@github.com:your-org/workspace-config.git --git ssh
```

### First Capsule

```bash
ws lift frontend my-feature
```

This creates a new branch `my-feature` from the default branch and drops you into the worktree. You're ready to work.

When you're done, push and open a PR. You don't need to clean up — once the PR is merged, `ws debrief` will detect that the capsule has landed and remove it automatically. If the PR is rejected or abandoned, `burn` it manually:

```bash
ws burn frontend my-feature
```

If you're standing inside the capsule you're burning, the [shell wrapper](#the-ws-wrapper) steps you out to the workspace root before the worktree is removed, so your shell is never left in a directory that no longer exists.

---

## Concepts

### Workspaces

A workspace is a directory containing a `ws.toml` config file and a `repos/` directory. The config file is typically stored in its own git repository, shared across a team, so everyone gets the same repo list, aliases, and display names.

`ws` discovers the workspace by walking up from your current directory looking for `ws.toml`. Every command works from anywhere inside the workspace tree.

### Ground

Each repo has a special worktree called `.ground`. This is the default branch — `main`, `develop`, whatever your team uses. Ground is the source of truth.

You don't work in ground. It exists so that new capsules can branch from a known-good state and so that `ws` can compare capsules against the baseline (ahead/behind counts, merge status).

Ground cannot be removed with `burn`.

### Capsules

A capsule is the unit of work in `ws`: one branch, one worktree, one concern. Capsules are cheap to create and cheap to destroy.

```
repos/frontend/
  .bare/            # the bare git repo
  .ground/          # default branch (sacred)
  my-feature/       # a capsule
  bugfix-login/     # another capsule
```

Each capsule is a full working copy. You can have as many as you need running simultaneously. When a capsule's branch is merged or goes stale, `debrief` cleans it up.

If a branch name contains slashes (e.g. `feature/my-thing`), the capsule directory uses only the last segment (`my-thing`) to avoid nested directories.

### Lifting

**Lifting** creates a new capsule — a fresh branch from ground.

```bash
ws lift <repo> <branch> [base]
```

- `repo` can be the canonical name, an alias, or `.` to infer from your current directory.
- `base` defaults to `origin/<default-branch>`. Pass a different ref to branch from somewhere else.

After lifting, `ws` runs the repo's `after_create` hook (if configured), boards the capsule into your IDE workspace, and `cd`s you into the new worktree.

### Docking

**Docking** checks out an existing branch into a new worktree. This is how you pick up work that already exists — someone else's feature branch, or a PR you want to review.

```bash
ws dock <repo> <branch>
```

You can also dock by PR number or full PR URL:

```bash
ws dock frontend 1234
ws dock https://github.com/your-org/frontend/pull/1234
```

When docking by PR number or URL, `ws` resolves the PR's head branch via the GitHub API and creates a worktree for it.

Like lifting, docking runs `after_create` hooks, boards the capsule, and `cd`s you in.

### Boarding

Boarding controls which capsules are visible in your IDE workspace files. When you `lift` or `dock` a capsule, it's automatically boarded. When you `burn` it, it's automatically unboarded.

You can also manage boarding manually:

```bash
ws board <repo> <capsule>
ws unboard <repo> <capsule>
```

Boarding state is stored in `ws.local.toml` and triggers regeneration of IDE workspace files (VS Code `.code-workspace`, IntelliJ project) so your editor reflects only the capsules you're actively working on.

### Debrief

Over time capsules accumulate. `debrief` scans all capsules across your workspace and cleans up the ones that are done:

```bash
ws debrief
```

A capsule is removed if it's:
- **Landed** — its PR has been merged, or (a repo with no pull requests) [`ws land`](#landing) landed it.
- **Inactive** — no commits for 90 days (configurable with `--days`).

A capsule is skipped if it has uncommitted changes, even if it's landed or inactive. `--burn-dirty-landed` removes a landed capsule all the same: its work is on the default branch, and what's uncommitted is lost.

Capsules that are still active are reported with their age, ahead/behind counts, and open PR status.

You can scope debrief to a single repo:

```bash
ws debrief frontend
```

### Landing

Most repos take work through a pull request. Some take commits straight to their default branch: a records repo of ADRs, proposals and notes, say. Say so, and `ws` lands a capsule there for you:

```toml
[repos.records]
landing = "direct"            # commits go straight to the default branch. Default: "pr"
verify = "just lint"          # optional: must pass in the capsule before anything's pushed
```

A repo can say it of itself, in its own [`ws.repo.toml`](#wsrepotoml) (`landing = "direct"`, `verify = "…"` at the top); `ws.toml` and `ws.local.toml` override that.

```bash
ws land                       # the capsule you're standing in
ws land note                  # a capsule of the repo you're standing in
ws land records note
```

`ws land` is the one place a landing is made, from a shell or the web UI:

1. It refuses a capsule with uncommitted changes, or with nothing to land.
2. It fetches, and rebases the capsule onto `origin/<default branch>`.
3. It runs the repo's `verify` command in the capsule, if it has one.
4. It pushes the checked commit to the default branch.
5. It brings the ground up to it (a fast-forward, never anything else), and burns the capsule.

It stops, with nothing pushed, where landing by hand should stop:

- **A rebase that clashes** stops with the capsule as it was. Rebase it there, settle the clash, and land again.
- **A check that fails** stops, with what it printed.
- **A number someone else took.** A document of a numbered [kind](#wstoml) that the capsule adds, whose number is on the default branch since (two people can take the same one), stops it, saying the next number and how to move the file to it.
- **A push that isn't a fast-forward** (someone landed first) is fetched, rebased, checked and tried again, twice. After that it stops and says so.
- **A push refused for any other reason** (branch protection, a hook) stops, with what the remote said. Nothing is ever forced. A repo that refuses probably isn't direct.
- There's never a merge commit.

**You are the gate.** Nothing stands between a landing and everyone else seeing it, so it's yours to make: `ws land` refuses crew and the orchestrator. Crew in a direct repo are told to commit, not push, and report done.

**In a mission,** a capsule of a direct repo with commits to land, and no crew in it, puts the mission in your court as `✓ land`. Its headline opens the capsule's review, which is the only review there'll be: approve it, then *Land on main* (`L`, then `⏎` to confirm). Approving and landing are two acts. A landed capsule counts as a merged pull request does: the mission is `landed` when every capsule has landed or merged, and a document in it goes from `draft` to `landed` with no `in review` between. A landing made by hand (`git push origin HEAD:main`) isn't recorded: land with `ws land`.

When `missions.review_before_land` is on, an approval carries across a clean rebase if the rebased tree matches the approved change applied to the new base. If it differs, review the rebased capsule before retrying. If Git accepts a push but ws can't record it, the capsule is kept. Retry `ws land` to check the remote and finish recording the same commit.

### Repos and Aliases

Every command that takes a repo argument goes through the same resolution pipeline:

1. **Exact match** against canonical names in `ws.toml`.
2. **Alias match** against aliases defined in `ws.toml` or `ws.local.toml`.
3. **Fuzzy match** — every character in your input must appear in order in the repo name (case-insensitive). If there's exactly one match, it's used. Multiple matches show an interactive picker.
4. **`.`** — infer the repo from your current directory.
5. **No argument** — interactive picker.

The same fuzzy matching applies to capsule arguments (e.g. in `burn` and `jump`).

Repos in hidden [groups](#groups) are always found by exact name or alias. Fuzzy matching only considers them when no visible repo matches, and the picker leaves them out.

### Groups

A repo can belong to one group. Groups let you keep repos in the workspace that you mostly read, such as another team's platform repos, without them crowding everything else. They also let you work with a slice of the workspace (`ws --group product`).

```toml
[groups.platform]
hidden = true

[repos.frontend]
group = "product"

[repos.platform-infra]
group = "platform"
aliases = ["infra"]
```

**Hidden groups** are left out of commands that cover every repo: `status`, `rg`, `debrief`, `setup`, `init` and `sync`. Naming a hidden repo always works (`ws jump infra`, `ws lift infra fix-alerts`, `ws rg --repo infra`), and shell completions include hidden repos.

- **Cloned on first use.** `init`, `setup` and `sync` don't clone hidden repos. The first `jump`, `lift`, `dock` or `rg` that needs one clones it.
- **Shown while you're working there.** A hidden repo with a capsule appears in status and debrief, marked `· platform (hidden)`. Once its capsules are debriefed, it drops out of view again.
- **Standing in one.** `ws rg` searches the hidden repo you're standing in, as well as the visible ones.

**`--group <name>`** limits a command to those groups, hidden or not. It is repeatable and comma-separated, and works on `ws status`, `mc`, `rg`, `debrief` and `setup`. `ws rg --group` adds the group's repos to any `--repo` selection. `ws setup --group platform` clones a whole group up front.

**Your own choice.** The team sets defaults in `ws.toml`. Each person can override them in `ws.local.toml`:

```bash
ws group                  # list groups, whether each is shown, and their repos
ws group show platform    # show it for you, and clone its repos
ws group hide product     # hide it for you
ws group reset platform   # go back to the team default
```

To hide a single repo for yourself, put it in a group locally (`[repos.mobile] group = "mine"`) and hide that group.

```bash
ws repo group mobile mine   # put it in a group, for you (ws.local.toml)
ws repo group mobile        # back in the team's group (a repo of yours: in none)
```

*Settings › Repos* does the same: each repo's group is picked in its row (one the workspace has, or *New group…*), marked *yours* where it's over the team's, and under the table each group is **Shown** or **Hidden** for you, with what the team says beside it and *Reset* to go back to it. Showing a group clones its repos that aren't, behind. A repo of the team's that the team has in a group can be put in another for you, or back in the team's, but not in none: `ws.local.toml` can't say "no group" over `ws.toml`'s.

### Repos of your own

A repo in `ws.local.toml` that `ws.toml` hasn't is **yours**: cloned by `setup` and `sync`, lifted, docked, searched and worked in by missions as any is, and not in a teammate's workspace. Say where it is on GitHub with `github` when it isn't the org's by that name:

```toml
# ws.local.toml
[repos.notes]
github = "someone/notes"
```

Its table is what makes it one, so `[repos.notes]` with nothing under it is a repo too (the org's `notes`). Settings' repos table marks it as yours, and *Share with the team* there does what `ws repo share` does. What's written about it reads for teammates as its address on GitHub, as they haven't the repo to link to.

```bash
ws repo share notes   # give it to the team: into ws.toml, committed on the config repo and pushed
```

`ws repo share` writes the repo into `ws.toml` as `ws.local.toml` says it, commits that on the workspace's config repo with *Add repo notes*, pushes it, and then takes it out of `ws.local.toml`. `ws sync` resets the config repo to its remote, so the commit is pushed or taken back: one the remote refuses (a protected branch) leaves `ws.toml` as it was and the repo yours. The config repo has to be at its remote, or behind it with nothing of its own (it's brought up first), and `ws.toml` mustn't have changes of yours that aren't committed.

### Adding a repo from GitHub

*Settings › Repos* lists the workspace's repos, the team's and yours, with where each is on GitHub. **Add repo…** (or *Add a repo from GitHub* in Find, there) picks one from GitHub: whose (you, or an org you're in, as `gh` sees them, or another you name), then which of theirs, newest pushed first, with forks and archived repos behind a box and those the workspace has already marked as here. What GitHub said is shown at once, however old, with when it said it, and asked again behind it when that's more than ten minutes ago (or on *Look again*); the list moves when it answers. An org that enforces SAML single sign-on lists nothing until `gh`'s token is authorised for it, and says so.

Chosen, it's given its name here (its name on GitHub, or, where that's taken here, `<owner>-<name>`, changed as you like), a [group](#groups) if you like, and whose it is: **just you**, written into `ws.local.toml`, or **the team**, written into `ws.toml`, committed on the workspace's config repo and pushed as `ws repo share` does (a push that's refused adds nothing). It's written with `github = "owner/name"` unless it's the org's repo of its name here, and cloned behind: the table says *cloning…*, then nothing, or *not cloned* with why (`ws setup` tries again).

```bash
ws repo add partner-co/api --as partner-api   # yours, as partner-api, cloned before it answers
ws repo add acme/billing --team               # the team's: into ws.toml, committed and pushed
```

### Searching

`ws rg` runs [ripgrep](https://github.com/BurntSushi/ripgrep) over the right worktree of each repo. Plain `rg` from the workspace root searches `.bare/`, `.silo/`, `.ground/` and every capsule, so every match comes back once per capsule. `ws rg` returns each match once, which matters most for agents whose context fills with duplicates.

```bash
ws rg useAuth                        # where I stand: my capsule for this repo, ground elsewhere
ws rg --repo fe,api useAuth          # restrict to repos (names, aliases, fuzzy)
ws rg --repo fe/login-fix TODO       # one specific capsule
ws rg --group platform Ingress       # a whole group, hidden or not
ws rg --ground -t go 'func Load'     # .ground everywhere
ws rg --all FEATURE_FLAG_X           # every worktree: who else is touching this?
ws rg -g 'src/**/*.ts' -l needle     # globs anchored per root
ws rg --roots                        # show what would be searched, run nothing
```

Everything that is not a `ws rg` flag is passed to ripgrep unchanged, so `-i`, `-t`, `-C`, `--json`, `--files` and friends all work. Scope flags are long-form only so they never shadow ripgrep's short flags. Parsing of scope flags stops at the first ripgrep argument or at `--`.

**Scope.** With no flags, the scope is "where I stand": if you are inside a capsule or `.ground`, that worktree is searched for its repo and `.ground` is searched for every other repo. Outside any worktree, `.ground` everywhere. `--ground` forces `.ground` for every repo, `--all` adds every capsule. `.bare` and `.silo` are never searched unless you name them explicitly (`--repo fe/.silo`).

`--repo` takes `name` or `name/worktree`, repeatable or comma-separated. Both halves resolve like any other repo or capsule argument, except that ambiguity is an error listing the candidates rather than an interactive picker, so the command is safe to call from scripts and agents.

**Output.** Paths are absolute, so each line says which repo and worktree the match came from, and you can open it from any directory. `ws` prints nothing of its own on success. Ripgrep's exit code is passed through: `0` matches, `1` none, `2` error.

**Globs.** Ripgrep matches a glob containing a `/` against the printed path, not against each search root, so a multi-root search would normally make `-g 'src/*.ts'` match nothing. `ws rg` expands such globs to one per root, keeping their anchoring. Basename globs (`-g '*.ts'`) pass through untouched. Globs from a ripgrep config file (`RIPGREP_CONFIG_PATH`) are not rewritten.

Do not pass positional paths; narrow with `--repo` and `-g` instead.

### Spaces of documents

A repo that's documents rather than code (a records repo of ADRs and proposals, a docs repo) is a **space**: `documents = true` under its `[repos.<name>]` in `ws.toml`, or at the top of its own `ws.repo.toml`. A folder of a code repo can be one (`documents = "docs/"`): its markdown under that folder. The records repo (`[missions] records`) is one without saying; `documents = false` says not, and `ws.local.toml` can turn any off for you.

- **Each space is read from its default branch,** as last pulled, into the workspace's records (`.ws/ws.db`): each document's title, its sections, and the documents it cites (by a link, or named as a ref). Only what a commit changed is read again: 220 documents read in about a second, and after that a look costs a `git rev-parse` a space.
- **`ws documents`** lists the spaces; `ws documents <repo>` a space's documents by folder, each by its title.
- **`ws documents search <words>`** finds documents by what they say, best first, each led to the section that says it best (`repo:path#heading`). A document whose title or a heading holds the words comes before one that mentions them; words are matched by their stems (*cache* finds *caching*), and the last word also as the start of one, as it's typed (*bin* finds *binary*; not once a space follows it, nor a small word or a single letter); a question can be asked in a sentence, the small words counting for little. It's words, not code: `AuditDispatcher` is one word to it, which `dispatcher` doesn't find (that's `ws rg`). `--space <repo>` keeps to one, `-f llm` is for agents, who may run it in a conversation.
- **`ws show <repo>:<path>`** of a document in a space says what cites it, and what it cites.
- **A space holds a plugin's things too.** A folder with a `thing.toml` in it is one, wherever it is in the space: the file says what kind of thing it is, what it's called, and which file in the folder is the thing ([Plugins](#plugins)). It's read with the documents, and again only when its `thing.toml` or that file changes. A folder with no `thing.toml` isn't one, whatever's in it.
- **A library.** A space that the workspace's [kinds of document](#wstoml) point into is listed by kind: each kind's documents with their number or date, title, where each stands (a kind with moves: *proposed*, *accepted*, filtered by it) and who changed it last, sorted by any of them; what's of no kind is under *Other*, by folder. *New* (`n`) in a space starts a page as one of its kinds; in a space with none, it asks for a folder and a title, and the page is to be filed there. *File as…* on any page can file it of no kind, in a folder of a repo (`repo:folder/`, its name made from its title, or a file's path), landed or left in a capsule as the repo lands its changes.
- **A space's heading** says how it stands with its remote (fetched when you open it, if it hasn't been for five minutes; *N commits behind origin/main* and *Pull*, a fast-forward of its ground, refused if the ground has changes of its own). Over its list, folded to a line (how many in flight, how many changes this week, the latest), is what's *in flight* (capsules of its repo with changes to its documents, and the pull request open from each, each document read as it is there) and what *changed lately* on its default branch: a line a change, which document, what was done to it, who and when, and what the commit said when it says more than that. A document in flight says so on its row, in the preview, and in the reader, which opens its version in the capsule (`?in=<capsule>`).
- **Searching in the web UI:** a box at the top of *Documents* (`/`), ranked as `ws documents search` is, each document led to the section that says it best, with its words marked; kept to a kind, and where one stands (*accepted build proposals about export*). A space's page has a box of its own (`/`), kept to the space: its lists keep to what's found, each row saying where in it (the section and its words), each kind's count and tab saying how many of its documents; the preview opens at that section. Find (`⌘K`) has the spaces' documents as a group of their own. A space's document, read, lists below it what cites it and what it cites.
- **In the web UI,** *Documents* (`g d`) takes the window's width: what *changed lately* and what's *in flight* across every space on top, then a card for each space (its kinds, each with how many, where they stand and which changed last, else its top folders; how it stands with its remote, what's in flight, your drafts in it) and, beside them, *Local* (your pages that aren't in a space). A space's page folds the sidebar to its rail and doesn't scroll as a page: its list (drafts, then each kind, then *Other* by folder; a tab for each kind over it jumps there, and follows as it scrolls) and, beside it, the document picked (`j`/`k`, or a click), as the reader shows it, with its sections and the changes to it lately, each scroll by themselves. `⏎`, a double click or *Read it* opens the reader; the edge between them is dragged wider or narrower (or `←`/`→` on it; a double click puts it back), and kept in the browser. The picked document is in the address (`?doc=`). On a narrow window it's the list alone, each document opened to read. A page that will live in a space is listed there as your draft, not in Local: one tied to a file there with something to save, one started as a kind whose path is there and not filed yet, and one filed there that hasn't landed. A page tied to a file with nothing to save isn't listed: the document is. In *Files*, a repo that's a space is folded into *In Documents*, at the foot of the list.

### Finding what's been said

`ws find <words>` finds what's been said by what it says, best first: in any mission (each decision, what was decided and why; its brief, section by section; its handoffs and other files in `files/`; its threads), in the workspace's conversations and passes, in pages (section by section, as each reads: its cells' words, not its questions), in artifacts (each one's title and its versions' notes), and in reviews (a mission's and a pull request's) and their comments. Each thing is found once, led to the place in it that says it best (a thread's turn, a section of a brief), with where it's read (a mission, the workspace, a pull request), how it stands, and its words marked.

```bash
ws find token cache                  # what ws records, then the spaces' documents
ws find "who coined hello world"     # a question, in a sentence
ws find sha256 -m house-sorting-cli  # only what's said in one mission
ws find webhook retries -f llm       # for agents: each with the command that shows it
```

- **Ranked as `ws documents search` is.** Words are matched whole and by their stems (*cache* finds *caching*), the last one also by its start (*bin* finds *binary*); every word counts for what it's worth (a rare one for more, *the* for nothing), so a question can be asked in a sentence. A thing whose title holds the words comes first (a decision is titled by what was decided, a mission's brief by the mission's title), more so the more nearly its title is just that; one that says every word that matters, before one that says some (and says so).
- **What's in play first.** What's of an archived mission, or a decision since superseded, comes a little below what's in play; a withdrawn decision not at all, unless `--withdrawn`. Ties go to the newer.
- **The spaces' documents are a group of their own,** the best five (`ws documents search` has the rest): their ranking is against one another, not against what ws records. `--mission` leaves them out.
- **Kept with the records.** What each says is written with it, in the same transaction (`.ws/ws.db`), so a decision is found the moment it's made. A mission's brief and its `files/`, a page's file, and an artifact's `artifact.toml` can be edited outside ws: each is kept with its size and time, and those (of missions in play, of pages and of artifacts) that have moved are read again before a search. A dismissed finding isn't searched, and what a cell's agent writes is found as the page's words, not the conversation's.
- **`ws doctor --reindex`** has every store say its words again from its records: what a change to what's searched needs. A workspace already in the records is read in when the daemon first starts after updating ws.
- **Agents** are told of it: crew, the orchestrator and conversations' agents may run it.
- **In the web UI,** Find (`⌘K`) has it as *Said*, after *Documents*: a line each, like a document's, with its words after its title and where it's read (a mission, a pull request) beside them; the card beside the list says what it is and where, with its words, those found marked. Choosing one goes to the thing's page: its mission (the *Brief* tab, for a section of the brief; *Summary*, for a decision), the review of its capsule or pull request, a handoff's page, the document a thread is on, the conversation, or the page a cell or a pass is of. `⇥` keeps to it (and to *Documents*, from one of theirs).

### Missions

A mission is a unit of work that the workspace owns: a brief, the capsules that carry it out (possibly across several repos), and a log of what happened. Capsules are still the building block. A mission just says which capsules belong to one piece of work, and remembers it after the agents and terminals that did the work are gone.

```bash
ws mission new "Fix token refresh race"     # creates it and opens the brief in $EDITOR
ws mission                                   # list missions (alias: ws mission list)
ws mission show fix-token-refresh-race       # brief, capsules, PRs, recent log
WS_MISSION=fix-token-refresh-race ws lift fe token-race   # new capsule joins the mission
ws mission attach -m fix-token-refresh-race api token-race   # add an existing capsule
ws mission note "Went with a mutex, not a channel"
```

**Which mission.** Commands that act on a mission take it from the slug argument or `-m/--mission`, then `$WS_MISSION`, then the mission owning the capsule you are standing in.

**`WS_MISSION`.** With it set, `ws lift` and `ws dock` attach the new capsule to that mission and print one extra `→ mission <slug>` line. `ws status -f llm` gains a short mission header. Without it, those commands behave exactly as they always have. Burning or debriefing a capsule that belongs to a mission logs the burn there. The capsule stays listed (as burned) so its PR still counts.

**State.** `draft` (nothing live yet), `active`, `review` (every capsule has a PR), and `landed` (every PR merged) are worked out from git and GitHub each time, so they never drift. A mission with a plan is where it is on its plan instead: `draft` until a step starts, then `active`, with the step it's at in words (*Step 2 of 7, yours: Is this the cause?*, *Step 1 of 7: Reproduce it as a failing test (c1)*), shown as *At* in `ws mission show` and on its page, and as *Step 2 of 7* in the list; its pull requests are facts its steps read, not its state, so merging them doesn't end it. A mission ends `landed`, `answered`, `handed-on` or `dropped` when you say (see *Endings*). `ws mission park`, `ws mission archive` and `ws mission reopen` set the two manual states; reopening a mission that's ended, and neither parked nor archived, takes its ending back. Archived missions are hidden from the list unless you pass `--all`. An archived mission is cleaned up: archiving burns its capsules. Once each one's pull request has merged at the commit the capsule is at (or it landed from there), with nothing uncommitted, that's one click. Otherwise archiving is refused, saying which haven't merged and why (*api/retry (it has moved since #88 merged)*), and you agree to burn them anyway (*Archive and burn* in the web UI, `ws mission archive --burn-all`): their branches are kept, and what's not committed is lost. It's refused while crew are at work on it, as ending it is. A burned capsule's branch is kept, so its pull request is still found; reopening the mission doesn't bring the capsule back.

**Brief.** `brief.md` is yours. `ws mission new --brief "..."` or `--brief-file path|-` writes it without an editor. Agents add to the log with `ws mission note` rather than editing the brief.

**Where you work on them.** Missions are run from the web UI (`ws web`; see *Web UI* below): the missions, what needs you, crew's conversations and terminals, review. `ws mission …` does the same from a shell, and is what agents use. (ws had a terminal UI for missions, `ws deck`; the web UI has taken its place.)

**Whose court it's in.** Every mission's ball is in one court: an agent's (crew, a briefing agent or the orchestrator is on it), yours, or outside (waiting on GitHub: reviewers, checks, a merge). Only an archived mission has none. Each mission has one **headline**: a sentence saying where it stands, and the one action that moves it on, even when that's "nothing for you to do". A mission's page leads with it, with a button for that action: going to crew that asked you something, want a permission or are waiting for your reply, answering a flag, opening a re-review, launching queued crew, resuming crew the orchestrator suggested, accepting a stage, opening a PR to merge, or archiving. The web UI and `ws mission show` give the same headline, worked out in one place.

The missions list groups missions by whose court they're in, each shown by its title with its headline's short label:

| Group | Court | When | Label |
|---|---|---|---|
| **Needs you** | yours | crew ask, want a permission or have finished their turn; the orchestrator asks, queued crew, or suggests resuming crew; a capsule is back for your re-review; a PR's checks fail, its review asks for changes, or it's ready to merge; crew report a stage done with the orchestrator off; a step of the plan is yours, or one waiting on CI or outside looks to have happened (every PR merged); an ending is suggested, or every step of the plan is done; a conversation about it proposed something to confirm; or crew have worked on it and nobody is now | `? asks`, `? permission`, `◐ waiting`, `! asks you`, `→ next crew`, `↻ resume`, `↻ re-review`, `✗ checks`, `✗ changes`, `✓ merge`, `✓ accept`, `✓ yours`, `✓ wrap up`, `↩ to confirm`, `idle` |
| **In progress** | an agent's | crew working, a briefing agent briefing, crew launching, the orchestrator deciding (from the moment crew report done or end by themselves) | `● working`, `● briefing`, `… starting`, `◆ deciding` |
| **Waiting** | outside | a PR waiting for review, or its checks running; a Wait step on someone else: one ws watches (it has a check), or one nobody's watching, until it's waited a day | `in review`, `◌ checks`, `⧗ watching`, `⧗ waiting` |
| **Up next** | yours, when you're ready | waiting to be briefed, briefed but never started, parked | `to brief`, `ready`, `parked` |

When several things apply, the headline is the most pressing: anything of yours first, then an agent at work, then waiting on others. While things are as they should be, that's one card on the mission's Summary: its title says what's happening (*Step 2 is yours to do: Tag and publish*), under it a sentence, and where there's nothing for you to do, a line saying so. ws says how each card is drawn, the same for every kind. What else applies is listed under it, as notices; one that's one thing to do keeps its button (ending the mission as suggested, opening a pull request, marking a Wait nobody's watching done), so it isn't held behind what leads the page, while a question's answers and a step's review are the lead's alone. A Wait step is never a second card under a question: the question's card carries the step's control first (*Look again* where ws watches it, *Mark step N done* where nobody does), then the answers, then *Answer…*, which opens a field for your own words. Every card keeps to one order: the one button that's meant to move it comes first, accented; what overrides it comes after, quieter. On a Wait ws watches that's *Look now* (*Look again*, once it has), with *Mark done anyway…* after it; on one nobody's watching, *Mark done*. A question's suggested answers are none ahead of another, so none is accented. A Review step leads with what's to review, then *Go on*, and *Send back…* opens its field; where the orchestrator asked about a Review step, the card is the step's, with *Dismiss the question* at its end. Under anything else, a Wait ws watches isn't drawn, since it passes by itself and the lead's step line and the plan say where it stands. PRs are looked at only where no crew are at work in their capsule: crew fixing red checks have them in hand.

Landed, ended and archived missions are behind *Show past* (`⇧A`). *Park* beside a mission's state puts it aside, and *Unpark* takes it back. Attaching and detaching capsules (`ws mission attach`, `ws mission detach`) and capsules that belong to no mission (`ws status`) are the shell's.

**Briefing.** A mission starts as a request: `fe issue 45`, a pasted issue URL, or a sentence, from a conversation whose agent proposes it, or `ws mission new "<request>"`. Until its brief and plan are confirmed it's an idea, *to brief* (there's no state for it: a mission raised from words is to brief until it's confirmed), and *Brief* on its page starts a briefing agent. Its only job is to turn your request into a brief and a plan you confirm:

1. Work out what you mean: repos and aliases from `ws status -f llm`, issues and PRs with `gh`, code in `.ground`.
2. Ask you about anything unclear.
3. Write the brief with `ws mission brief --set -` (its purpose: goal, context, constraints, acceptance), and name the mission with `ws mission title`: a terse, plain-English name of at most 50 characters, which is how it reads in the missions list.
4. Write the plan, from the brief and the workspace's process, with `ws mission plan --set -` (see *The plan*), asking you where you want to check in.
5. Dock existing work it builds on, if any (`ws dock`): capsules aren't the brief's. The plan's steps name their repos, and ws lifts a capsule when each starts.
6. Check with you, then run `ws mission ready`.

The briefing is a conversation in the web UI, on the mission's *Brief* tab (where *Brief it* goes): the conversation with the agent, and beside it, once there's a draft, the brief and the plan as documents (see *The Brief tab* below). The agent asks there, and proposes the brief and its plan rather than writing them; each draft after the first is shown in them as tracked changes since the one before, to accept or decline passage by passage, and then to edit in place. Nothing's written until you confirm. **Confirming its proposal is the go-ahead** (*Confirm and start*, or `mod+⏎`): the mission is recorded confirmed, by you, and starts at once: ws reaches the plan's first step (its crew start, or the step waits for a call). `in = "terminal"` under `[briefing]` runs it as crew in a terminal instead, as described above; so does having no harness that can hold a conversation. Agents that suggest a mission (`ws mission new`) brief it this way too, without a conversation.

**A brief can start small.** "Find out why invoice totals are off by a penny" is a clear brief, its acceptance the cause named with where and why, and its plan finds that, then plans the rest (a *Plan* step). When what's found changes what the mission is for, the planner proposes its new brief (goal and acceptance) and the plan to get there, as one proposal in its conversation. A change to the brief is always yours to confirm, and confirming it is the go-ahead: the mission moves on (past the planner's step, or the orchestrator looks at the plan as it is now). Like any revision, it can't change the steps behind the mission.

`ws mission ready` refuses until the brief is written and, for a briefing agent, the plan is (`--no-plan` to go without; then a capsule has to exist, `--no-capsules` if none is needed), can rename the mission (`--slug`), records the briefing agent as `briefed` and ends its session, and confirms the brief: the mission starts as above. A mission without a plan waits for you to launch its crew. A briefing agent asking you something puts its mission under **Needs you**; one nobody has started briefing waits under **Up next**. Stop a briefing early and its page offers to resume it.

`ws mission new "<what it is>"` works from an agent too: the mission is an idea, to brief. Crew that spot something outside their mission are taught to raise it this way; the new mission records which mission and crew raised it. Raised from inside a mission (crew, or the orchestrator), one that looks like a mission in play is refused, naming it, so what was found is noted there (`ws mission note -m <mission>`) rather than raised twice; `--distinct` raises it anyway, for different work. Alike means sharing names from the code (paths, identifiers, test names, environment variables), not just words. To create a mission ready, with a brief you write yourself, pass `--brief`, `--brief-file`, `--edit` (opens `$EDITOR`) or `--no-edit` (the template).

The same commands work by hand: `ws mission brief` prints or replaces a brief, `ws mission plan` its plan, `ws mission title` renames (at most 50 characters), and `ws mission ready` finishes a briefing. Crew can only change the brief or title while briefing; after that they belong to you (and the orchestrator, which names a mission still titled with the request it was raised with).

**Crew.** *Launch crew* on a mission's page starts an agent on it (with crew already at work, the headline goes to them; launching more is beside it). It starts in one of the mission's capsules, with `WS_MISSION` and `WS_CREW` set, and is told to read the mission with `ws mission show`, so the capsules it lifts join the mission. Each launch is recorded on the mission as crew (`c1`, `c2`, …) with how it ended. Crew get `.ws/bin` first on `PATH`, where `ws` and `workspace` point at the binary that launched them, so the commands they're told to run always match the version running them. Crew and the orchestrator also get `GIT_CEILING_DIRECTORIES` set to the workspace's `repos` folder: `repos/<repo>/` holds a repo's clone and capsules but isn't a repository, and git run there would otherwise answer for the workspace's own config repo (an agent once took its commit for the repo's). `ws mission show -f llm` lists each repo's default branch, the commit its `.ground` is at, and where to read it.

- **Several crew.** A mission's page lists its crew, each saying what it's doing and what it was dispatched for; each has a page of its own (see *A crew's page* below). Launching more puts them in a capsule nobody is working in, and refuses when every capsule has crew at work: two agents in one worktree edit the same files. Crew that have reported done (`✓`), or finished briefing, have handed their work back and don't count as working: launching in their capsule ends their session first (logged; their page resumes it later).
- **Working or waiting.** A crew says what the agent is doing: working, waiting (it has finished its turn and is waiting for you), or asking for a permission. ws knows this from the agent itself: it installs hooks in each crew session it launches (Claude Code through `--settings`, Cursor through a plugin, pi through an extension, each for that session only, alongside your own), and the agent reports when you've sent it a message (working), when its turn ends (waiting, with what it said, which the mission's Summary and the notification show), when it asks you something with its question tool (asking, with the question and its options, rather than "needs your permission": Claude Code's, and Cursor's on the models that offer it, which `ws doctor --fix` enables as it does Cursor's other hooks), and, for Claude Code, when it needs permission (for pi, when one of your extensions' dialogs is waiting for an answer). Where there are no hooks (the `shell` harness) ws judges from the screen: still for 5 seconds means waiting. Whenever a mission's ball lands in your court (crew go quiet, ask or want a permission, the orchestrator flags you or queues crew, a capsule comes back for re-review, a PR is ready to merge…), the web UI's tab icon, ws's eclipse, lights up amber, and, if you've turned them on, a desktop notification while the tab is in the background.
- **Cursor's hooks need one thing from you.** Cursor runs a plugin's hooks for an event only if your own `~/.cursor/hooks.json` has a hook for it too. `ws doctor` checks this (once missions exist and the Cursor CLI is installed), and `ws doctor --fix` adds hooks for `beforeSubmitPrompt` and `stop` that do nothing (`/usr/bin/true`), keeping everything already there. Without them, ws still learns Cursor crew's session id, but judges working or waiting from the screen.
- **Ask and done.** Crew are taught to say where they are. `ws mission ask "<question>"` marks them `?c2 asks` with the question; `ws mission done "<summary>" --handoff -` marks them `✓c2 done` (see Handoffs below), and hands the ball to the orchestrator. Each stands until you send that crew a message. A crew asking or waiting moves its mission under **Needs you**. Crew are taught not to report done while a question of theirs is open.
- **Artifacts.** The mission view lists what the mission has produced (its *Outputs*, in the web UI): each capsule's PR (found from its branch, so a mission in several repos lists one per capsule), then the artifacts recorded on it, grouped by kind, each with where it is now. Once every PR is merged it says the mission has concluded, and *Archive* archives it. See [Artifacts](#artifacts) below.
- **Harnesses.** A harness is the agent CLI crew run in: `cursor` (the Cursor CLI, the default), `claude` (Claude Code), `pi` (the pi coding agent) or `shell` (your `$SHELL`). Crew run the one configured for crew (see [Agents](#agents) below). If the mission's last crew can be resumed (the agent reported a conversation id), the mission's page offers to resume it beside launching new crew. Resuming continues the same crew: same `c1`, same conversation and harness, back in the capsule it worked in.
- **The daemon.** Crew run in a per-workspace daemon, not in whatever launched them, so they keep working when you close the browser or the terminal. It also runs the orchestrator and conversations' agents (one run at a time per mission or conversation), serves the web UI and watches the missions' PRs. It starts when it's first needed (a launch, `ws web`), logs to `.ws/daemon.log`, and stays up until it's closed, however long nothing's running or open (a laptop's sleep doesn't take it down). `ws daemon status` shows what it holds and which ws it is; `ws daemon stop` closes it, hanging every crew up. It keeps the ws it started from: when another's installed in its place (`ws upgrade`, a rebuild), it finds it within seconds and starts again on it once nothing's at work, which is no crew mid-turn and no agent's run (the orchestrator's, a check's, a conversation's answer). Until then `ws daemon status` says which ws it's to restart onto and what it's waiting for. `ws daemon restart` asks for the same, and `ws daemon restart --now` doesn't wait. Restarted, it's the same process on the new ws: its environment and the web UI's link are as they were. Stopping and restarting it are yours to do: crew and the orchestrator are refused. Crew it hangs up that nobody asked to stop (a restart, an upgrade, `ws daemon stop` itself) are recorded as interrupted, and the next daemon resumes those still at their work as it starts: crew that were mid-turn are told the daemon shut down under them and to carry on, crew that were waiting for you are reopened as they were. Crew that had handed their work back, whose capsule later crew have worked in since, or whose mission has ended or is parked are left as they are. Each is looked at once, and the daemon's log says what it did with each. It also keeps the workspace's records, `.ws/ws.db`, which only it opens: a command that reads or writes one asks it over `.ws/store.sock`, starting it if it's down. A command in a sandbox that closes sockets (Codex's) has to run outside it. Conversations and threads are kept there (a run's transcript stays a file, `.ws/conversations/w4.run`, or in its mission's `conversations/`). The first time the daemon starts each day it copies the records to `.ws/backup/ws-<date>.db`, keeping the last three. `ws doctor` checks the file is sound, says when the workspace is somewhere SQLite's write-ahead log isn't safe (a network disk, or a folder iCloud Drive, Dropbox or the like copies), and finds files under `.ws` that no record has (what a write that didn't finish left, or what was kept before the records): `ws doctor --fix` moves them aside, to `.ws/swept/<date>/`, where they can be put back from.
- If the daemon dies, crew it left recorded as running are marked `lost` when it next starts, and an orchestrator run it was in the middle of is run again.

**Orchestrator.** When crew report done (at a Work step with checks, once ws has checked it and found something unmet: see *Checks*), a Work step runs out of time, crew end by themselves (not when you stop them), finish a turn without asking or reporting done after a message ws sent them (their opening prompt, or the orchestrator's nudge), or one of the mission's PRs fails its checks, gets changes requested or merges while no crew are at work in its capsule, ws runs the orchestrator: a headless agent (`cursor-agent -p`, `claude -p` or `pi -p`) that reads the brief, the log, the crew so far, each capsule's diff and PR checks, and the end of the latest crew's output, then decides what happens next. There's no fixed plan: if the brief says "write a proposal and commit it to docs, then a new agent implements it", the orchestrator sees from the history that the proposal is done and dispatches the implementation. It acts only through `ws` commands, so a mistake comes back as an error it corrects:

| Command | Means | In the web UI |
|---|---|---|
| `ws mission note "Orchestrator: …"` | What it found and decided, always one | In the log, read by its next run |
| `ws mission accept "<why>"` | The current stage's (or plan's step's) work is complete and good enough: the next becomes current (the why is optional, and recorded) | Stages, or the plan, on the mission's Summary |
| `ws mission dispatch "<what to do>" [--capsule repo/name] [--stage n]` | Start fresh crew on the next piece of work, on the current stage (or stage n) | Crew appears under the mission, with *joined (dispatched)* and a notification |
| `ws mission resume-with <crew> "<what's left>"` | That crew should carry on | Crew still running get it typed straight in (it counts toward the dispatch limit). Ended crew: the mission's page offers to resume it, and types the message in |
| `ws mission pr-ready [<repo>]` | The Open Pull Request step's draft, its checks green, is ready for review (as the process says) | The pull request is marked ready on GitHub; held by `review_before_ready` |
| `ws mission start [--in <repo>]…` | The step waiting for its call starts (one that applies only if, or runs in each of the repos named) | The step under way, with its runs |
| `ws mission skip "<why>"` | The step waiting for its call doesn't apply (its only if doesn't hold) | The step marked skipped, with why |
| `ws mission repeat [--in <repo>] "<why>"` | The Work step starts again with fresh crew, another attempt, within `max_rounds` | New crew on the step; its attempt on the mission's page |
| `ws mission limit [--step n] <45m\|1h>` | The step has longer, from now (its time ran out, and the process says crew may have more) | The step's clock on the mission's page |
| `ws mission flag "<question>" --option "<answer>"…` | Something only you can decide, or the brief says to check with you first | A task of yours: `! asks you` on the mission, the question and its answers on its page, with a box for your own and *Ask an agent*; your answer goes to the orchestrator, which acts on it. One open at a time: asking again takes its place. *Dismiss* dismisses |
| `ws mission new "<follow-up>"` | Separate work worth doing | A new mission waiting to be briefed, raised from this one |
| `ws mission conclude "<what it came to>" --as <ending> --met "<n>: <how>" --unmet "<n>: <why not>"` | How it ends (landed, answered, handed-on, dropped), each part of Z checked | `✓ wrap up` on the mission: the ending and Z as checked, *End it* ends it (see *Endings*) |
| `ws mission hand-on <repo>/<capsule> "<what for>"` | A capsule's work belongs in a mission of its own (as a plan may say of a repo that fails) | A mission raised for it, which takes the capsule; this one lists it as handed on |
| `ws mission replan "<what doesn't fit>"` | The plan no longer fits what's happened (once at a step; after that it flags you) | The planner at work in its conversation; its revision applies, with the log saying what changed, or comes to you when it touches a checkpoint or Z (see *Revising*) |
| `ws mission wait "<what it waits on>"` | Nothing should happen until something ws can't see does (a teammate, another team's deploy) | `⧗ waiting` under **Waiting**: nothing for you to do. Anything happening on the mission clears it. Refused at a Wait step of the plan, which says what it waits on itself, with marking it done on its page |

**Watching PRs.** The daemon looks at the missions' PRs every two minutes while it's running (it shares what GitHub said with the web UI). Each capsule's PR is recorded as last seen, so a change is noticed once, and also when it happened while nothing was running: the next look catches it. Each change is logged as an event of its kind (*api#88's checks are failing*, *api#88 conflicts with its base*, *api#88 merged*, *api#88 was closed without merging*). A PR at its Open Pull Request step is that step's (see *Looking after a pull request* under *The plan*). Any other: red checks, a conflict, requested changes, a merge and closing it run the orchestrator, unless crew are at work in its capsule; approvals and passing checks only change the headline (*ready to merge*), and so does the merge of one whose Open Pull Request step was passed already. Parked and archived missions aren't watched.

A pull request's checks are failing when the latest run of any of them failed. GitHub lists every run made on its head commit: a check that failed and then ran again on the same commit (the pull request was edited, and its title checked a second time) counts as its later run.

A run that runs no `ws mission` command at all (a model that takes one refused command for "the shell is denied" prints what it would run instead) is run once more, told so and that its shell works. If it still does nothing, or can't run at all (a sign-in that's run out, say), ws flags you with what it said, so the mission is yours rather than nobody's; answering runs it again with your answer.

Every run leaves the mission in someone's court: crew carrying on, a question for you, a stage accepted and the next started, a suggested conclusion, or waiting on others. It's told where the mission stands if it does nothing; a note alone is enough only when that's already yours for something ws shows you (crew asking, a re-review, a PR to merge), and otherwise leaves you a mission nobody's working on, with its note. Whose message crew finished their turn on decides who looks: after yours (from its page, or Enter in its terminal), it's answering you, so it's yours; after ws's, the orchestrator looks first, so a crew that stops to say "shall I write tests too?" gets an answer or a nudge, or the question comes to you as a flag.

**What it waits on you for: tasks.** A mission keeps what it waits on you for as tasks, a queue, oldest first: a question the orchestrator (or crew) asked you, and each step of the plan that's yours (a Review to review, a Decide to answer, a Land to land, a Wait on you to do) or that ws waits on you to start (a stage to go on to, a pull request to open once you've reviewed it). The oldest is the mission's headline, and each has its own: a question asked while a step is yours doesn't take the step's place. A step has one task, asked again in its place; each asker has one open question, asking again in its place. `ws mission show -f llm` lists them by number, and the orchestrator is told them, so it doesn't ask what's asked already. A mission from before tasks has its one question (its flag) as a task.

**Answering.** `ws mission answer "<text>"` answers the oldest task, `--task <n>` another; the mission's page answers the one its headline is. A question's answer is logged as a decision, counts as you stepping in (the dispatch count starts again), and runs the orchestrator with your question and answer, so it dispatches, resumes or concludes as you said. At a step of yours, *Go on* passes it (or starts it, where ws was waiting on you); *Not yet* leaves it yours (a stage ws asked to start waits, and launching crew starts it); anything else is logged as a decision and goes to the orchestrator, the step staying yours. A Decide step's answer is the decision, and passes it. At a Wait on you, *Done* passes it (after ws's look, where it's watched), alone or with what you did after it. Questions usually come with two or three answers the orchestrator would act on: the page offers those, a box for your own, *Ask an agent* and *Dismiss*. Asking an agent starts crew (the crew harness and model, in the workspace root) whose one job is the oldest question: it's told the question and the suggested answers, not the whole brief; it finds out what's needed (runs the tests the orchestrator can't, reads the capsules) and answers with `ws mission answer`, which counts as your answer. It doesn't do the work the answer leads to: the orchestrator dispatches that. While it works, the mission shows it working rather than asking you. *Dismiss* (or `ws mission flag --clear`) puts aside the oldest question (or a stage ws asked to start), not a step of yours: go on, or change the plan. The orchestrator doesn't flag what crew already asked you: you answer the crew. With `[orchestrator] enabled = false`, answering a question logs it, and the rest is up to you.

**Dispatch.** Dispatched crew start fresh, told the brief, what they're dispatched to do (where earlier crew's output is, what to watch for) and where earlier crew left off (see Handoffs below), and do only that part before reporting done. They run in the daemon, so the orchestrator can move a mission on with nothing open; the web UI shows them. If the next crew needs a capsule the mission doesn't have, the orchestrator lifts one first. You can dispatch by hand too (`ws mission dispatch -m <slug> "<what to do>"`). Briefing agents write a plan (below), with your checkpoints in it; a brief written by hand can still list `## Stages`.

**The plan.** A mission's route from A to Z, `plan.md` beside its brief: what Z is (what has to be true for the brief's purpose to be met), where it may stop early, and its steps in order, each one of the kinds of step ws knows. The briefing agent (the planner) writes it from the brief and the workspace's process, as it's written (see *The process*), following the route the process gives the kind of work this is, and you confirm it with the brief. ws reads it strictly and refuses one that isn't right, saying every problem and its line (`ws mission plan --format` says how it's written, with every kind of step and what each takes):

```markdown
## Z
- Merged, with its regression test.

## Stops early
- It can't be reproduced, or isn't a bug: that's an answer for product.

## Steps
1. Work: reproduce it as a failing test, and find the cause. "Bugs are reproduced first"
   - in: `api`
   - limit: 1h, less if you like
   - makes: the failing test, and the cause
   - check: `go test ./invoice -run TestTotalRounding` fails
   - check: the test fails because of the cause the handoff names
2. Review: the failing test, and is this the cause?
3. Work: fix it.
   - carrying on from: 1
   - on: claude:haiku
   - check: `go test ./...` passes
   - check: it fixes the cause, not the symptom
4. Review: is the fix right?
5. Open Pull Request
   - in: `api`
6. Decide: does it go to pre and prd now?
   - only if: it changes what pre and prd run
```

- **Kinds of step.** Each step starts with its kind, then what it's for after a colon; under it go its parameters, only those its kind takes. ws does the rest of each step, the same for every mission:
  - **Work**: crew do a piece of work, `in` a repo (`api`: ws lifts a capsule there for the mission, if it hasn't one, when the step starts) or one of the mission's capsules (`api/invoice-total`), `on` a harness and model if the step says, and what it `makes`. Fresh crew, unless it's `carrying on from` an earlier Work step: then that step's crew carry on, told the new step (typed in if they're still running, resumed if they've ended; fresh crew told to read their handoff if they can't be). With no `check`s, done when they report done, as the orchestrator judges; with checks, see *Checks* below. A `limit` puts it on a clock (see *The limit*).
  - **Review**: yours, a checkpoint: a review of what the steps before it made, back to your last step (the capsules their crew worked in, and the documents made there). What the plan put under review is yours to approve: those capsules, and a document the plan names by its path (in the Review step, or in the text, `makes:` or checks of a step it looks at). Crew's other documents, their notes and handoffs, are to read beside it. Its task is a review (*Step 2 is yours, to review: the fix. Review api/fix: approving it passes the step, and asking for changes sends it back to the crew who made it. Or go on. The crew's notes on it, to read: docs/why.md.*), with *Review api/fix* and *Read why.md* on the mission's page (`v` opens the first). Your reviews approving what's to approve as it is now (each capsule at its head, each named document as it reads) pass the step; a review asking for changes goes to the orchestrator, which sends it back to the crew who made it, and the step stays yours, to review again. *Go on* passes it as it is; *Send back*, with what's to change in your words, goes back to the crew the same way. Where the steps before made nothing to approve (findings in words, or only notes), you read what they found and go on.
  - **Decide**: yours, a question (`from` where the options are, if they're written down). Your answer is recorded as a decision, and passes the step.
  - **Open Pull Request**: ws's own. It opens the pull request from the mission's capsule in the step's repo (a `draft` if the step says), written by the PR writer (below), holding the guarantees (with `review_before_pr` on, it asks for your review first, and opens it once your review approves it). It doesn't open one that would conflict from the start: when the capsule doesn't merge cleanly onto its base (main moved on while it was built), it goes back to the crew who worked in it, told what conflicts and to merge main in (not rebase), and their done opens it; past `max_rounds`, or with no crew to send it to, it's yours to merge, and going on opens it. Opening one by hand or by crew (`ws mission open-pr`) is refused the same way, naming what conflicts. It looks after it while it's open (below), and the step passes when it merges; the orchestrator can't pass it before. Crew at other steps commit and don't open one.
  - **Land**: yours, for a repo with no pull requests: ws asks you to land the capsule, and the step passes once it's landed.
  - **Wait**: for something that happens outside the mission. It says who it's on (`- on:`), and may have checks (`- check:`), each a command that shows it's happened. Nothing passes a Wait by itself unless it has a check, and the mission's page says which it is, *Watched* or *Manual*:
    - **On you** (`- on: you`): something you do yourself (cut a release tag, approve a publish). It's yours, under **Needs you** (`→ to do`): ws asks you, with what the plan says and what the crew before it left, and *Mark done* passes it (`ws mission answer "Done"`). Say what you did with it (`Done: tagged v1.4.0`) and that's kept as a decision, which the crew after are told. Only you pass it.
    - **On anyone else** (`- on: the platform team`): it waits, under **Waiting**.
    - **Watched** (it has a check): ws runs its commands every minute at first, then every five, then every fifteen once it's waited two hours (in the repo its `- in:` names: the mission's capsule there, else the repo's ground; the workspace's root without one), each stopped after a minute, and passes the step once every one is met. *Look now* on the page (`ws mission look`) runs them at once. Marking a watched step of yours done has ws look first: not met, it stays yours, with what the look found, and *Pass it as it is* passes it anyway.
    - **Manual** (no check): nobody's watching, and nothing wakes the orchestrator for it. It's passed when someone says it's happened: *Mark done* on the page, or `ws mission accept "<what happened>"`, whose words are kept as a decision. One on someone else comes to **Needs you** once every pull request has merged and every capsule landed, or it's waited a day (`? happened`), to ask whether it has.
    - What the orchestrator asked you at a Wait (its run failed: again, or wait?) is closed when the step passes, marked done or seen by ws's look, the log saying why: it isn't left to lead the next step's page.
    - A plan written now has to say who each Wait ahead is on; one written before a Wait said still reads, as a Manual wait on someone outside.
  - **Plan**: the planner writes the steps after it (see *Revising*).
- **The orchestrator's calls.** There's no branching in a plan: when a step applies, and where it runs, are words, which the orchestrator applies, and ws records what it decided.
  - **`only if:`** (any kind of step): it doesn't always apply. Reached, it waits for a call: the orchestrator is run (trigger `step`), and starts it (`ws mission start`) or skips it, saying why (`ws mission skip "<why>"`), by what the steps before made and the process. A step without `only if` always runs: the orchestrator can't skip it (if the plan no longer fits, it asks the planner), and can't accept a step that's waiting for its call. With the orchestrator off, it's yours: *Go on* or *Skip it*. A skipped step shows as skipped, with why (`ws show s2`, the mission's page).
  - **`in each:`** (Work and Open Pull Request, in place of `in:`): it runs in several repos, said in words (*repo of the workspace*, *repo where step 1 made changes*). Reached, the orchestrator names them (`ws mission start --in api --in web`), and it runs once in each: crew in each repo's capsule (lifted where the mission has none), or a pull request from each. Each run is done when its crew report done (with its checks met, where it has them), or its pull request merges; the step is done when every run is, and the orchestrator can't accept it before. A step that carries on from one that ran in several repos runs in the same ones, each run's crew carrying on. Starting a step's runs counts as one dispatch toward the crew ws starts without you; with `[orchestrator] dispatch = "ask"`, they're queued one at a time, the next as each is launched. With the orchestrator off, you name the repos in your answer (*api, web*).
  - **`ws mission repeat "<why>"`**: the Work step the mission's at, started again with fresh crew, another attempt (`--in <repo>` for one run of a step in several), when resuming the crew won't do. The orchestrator may, within `[missions] max_rounds` attempts; after that, it's yours.
- **Looking after a pull request.** While an Open Pull Request step's pull request is open, what happens to it is the step's, and how it's looked after is the process's to say (*Tend to my pull requests: fix failing checks, and consider review comments…*), which the orchestrator reads:
  - **Red checks, or a conflict:** a round of fixing, put to the orchestrator (trigger `pr`, told what happened since its last run and the round), which resumes the crew who worked in its capsule with what failed, or asks you, as the process says. After `[missions] max_rounds` rounds it's yours: a question, *Fix it* or *Leave it to me*. Crew at work in the capsule have it in hand.
  - **A draft whose checks pass:** with `review_before_ready` on, your review first: ws asks for it, and marks the draft ready once your review approves the capsule as it is. Otherwise it's the orchestrator's call by the process: mark it ready (`ws mission pr-ready`), or have you review it first. With the orchestrator off, it's yours: *Go on* marks it ready.
  - **What's said on it:** a review with something to say, a comment, or a comment in a review thread, by anyone but you, is news, logged with who said it and where (*bob on api#12, `cache.go:12` (thread PRRT_kw1): Why a map?*), and the orchestrator's to act on by the process: crew resumed to fix what's asked, or to draft a reply, or you asked. A bot's comment on the pull request itself (a coverage summary, a preview's link) is logged and runs nobody: its review, or its comment on a line, does.
  - **Replies:** crew draft one with `ws mission reply api#12 --thread <id> "<text>"` (a review thread), or `--to <login>` (the pull request's own conversation). ws posts it on GitHub as you. A reply to a person waits for you first (`replies_need_me`, on unless set off): a task, *Post it* or *Don't post*, with the reply in it. A reply to a bot is posted at once.
  - **Closed without merging:** the orchestrator's call; with it off, yours.
  - **Merged:** the step passes, and what ws or the orchestrator asked you at the step (whether to merge it, what to do about a check) is closed with it, the log saying why. A question from before the step, or crew's, stays yours.
- **Checks.** A Work step's `check:` lines (as many as it needs) are what has to be true for it to be done. When its crew report done, ws checks the work, before the orchestrator or you look at it (the headline says *ws is checking step 1*): a check that's a command in backticks followed by `passes` or `fails` (`` `go test ./...` passes ``) ws runs in the step's capsule (in the daemon, up to 15 minutes, as you would from a shell), met when it exits as the check says, can't tell when it can't run at all; anything else, in words, the judge judges (below). A command is run as a task (see [Runs](#runs)), so it's kept: the check goes to its run, with all it printed, how long it took and the commit it ran at, where the check itself shows the end. A check that's `` `ws run <recipe>` passes ``, for a task the capsule's repo has a recipe for, is that recipe's own run, with its reports. Each is *met*, *unmet* or *can't tell*, with what shows it (the end of the command's output, or the judge's words), and every round is kept on the step (`ws show s1`, the step on the mission's page, the log). **All met:** ws passes the step, and reaches the next, without waking the orchestrator. **Unmet:** the orchestrator is run with what's unmet (trigger `checks`), and decides by the process: back to the crew (`ws mission resume-with`), or yours. It can't pass the step itself, and the planner can't change a step that's begun, so where a check is unmet only because you've decided otherwise since, it asks you rather than sending the work back to go against you. Its question at such a step (`ws mission flag`) is the step's own: *Pass it* and *Send it back* come before its own answers, so you pass the step from the question. After `[missions] max_rounds` rounds unmet (3 unless set: see *Guarantees*), or with the orchestrator off, ws asks you itself: a task, *Pass it* (as it is) or *Send it back* (to the crew whose done it checked, with what's unmet), or your own words for the orchestrator. Answered in your own words, what you say stands over the checks: the orchestrator is run with it, and may then pass the step where that's what you said. **Can't tell:** yours, the same. Where crew handed over a bundle for the step, the task goes to it (*Verify b1*: see *Verification bundles*), and approving that bundle passes the step, asked or not. **Left:** a step whose checks came round unmet, with nobody on it and nothing asked (the orchestrator only noted, or you dismissed its question), is yours on the mission's page, which says what the round came to and offers *Pass step 1 as it is*. You can always pass the step yourself (`ws mission accept`). A plan revision that adds a command ws hasn't been given to run before is yours to confirm, as a checkpoint removed is.
- **The judge** judges the checks in words: an agent that didn't do the work, run once, headless and read-only (it may read the capsule, and `ws show`, but change nothing), told the step, its checks, the crew's handoff, what ws ran, the mission's documents and the capsule's diff, and what you've decided since the step's words could last change (since its crew started on it, or since the plan was last written if that was before). A decision of yours stands over the step's words: where it drops or changes what a check asks, the judge judges the check as your decision leaves it, and names the decision. Only yours are given: what you answer crew who asked you something (`ws mission ask`) is kept as a decision of yours, with their question as its why, and crew's own record of what you chose doesn't change a check. A check it speaks of twice is read by what it said last. It runs on `[judge]`'s harness and model (in `ws.toml`, `ws.local.toml` or Settings), else the planner's: briefing's agent and model. One that can't be asked makes its checks *can't tell*. Its latest run is kept beside the mission (`judge.log`).
- **The limit.** A Work step's `limit:` (`45m`, `1h`, `1h30m`, with words after it if they help) is how long its crew have: ws holds the clock from when they start on it. Crew may give themselves less (`ws mission limit 45m`), never more; the orchestrator and you may give it more (`ws mission limit [--step n] 1h`; with no duration, the step's limit again). When it runs out while crew are still at it, the daemon says so, once (it looks every minute): the orchestrator is run (trigger `limit`) and decides by the process (more time, have them wrap up, or yours); with it off, it's yours, a task: *More time* or *Wrap up* (ws tells them to commit what they have and report done with what they found).
- **Before kinds of step,** a plan said who's on each step (``- who: crew, in `api/x` ``, `you`, `CI`, `outside`, `the planner`) and what it makes; one written so still reads, each step the kind its who: is (crew Work, you Review, CI and outside Wait, the planner Plan), and a crew step that makes a pull request has its crew open it, through ws, as before.
- A step the process asks for ends with the process's words in double quotes, which have to be in `PROCESS.md` as it is when the plan's written. A step that's yours (Review, Decide, Land) is a **checkpoint**: nothing else passes it.
- **Written** while briefing: in a conversation, the agent proposes the brief (`ws mission brief --propose`), then the plan (`ws mission plan --propose --set -`), which comes with the brief as one proposal of both: the mission's *Brief* tab shows the brief and the plan (as does its card in the conversation), and *Confirm and start* writes both, as you left them, and starts it. Proposing the brief again keeps its plan. A briefing agent in a terminal writes it with `ws mission plan --set -`, and `ws mission ready` wants it there (`--no-plan` to go without). The plan is yours, and the planner's while briefing: crew and the orchestrator can't change it. On the mission's *Brief* tab (`e` goes there) it's read, with *Rewrite from the brief* to have the planner write it again and *Edit by hand* to change it yourself, where a plan ws can't read is refused with every problem shown under it; or it's set with `ws mission plan --set -`; in the mission's *Talk*, its agent can propose an edit to it; once it's briefed, the planner revises it (see *Revising*).
- **Followed.** A mission with a plan works from its steps in place of its brief's stages: they're its stages to ws (the progress, `ws mission accept`, `--stage n`, `ws show s2`). When a step is accepted, ws reaches the next by its kind, as above. The orchestrator is given the plan (Z, where it stops early, and the steps with their kind, who's on each and their progress) in place of the stages, and a shorter prompt than a mission without one: ws holds the route, refusing what isn't the orchestrator's (passing a step of yours or the planner's, dispatching crew to a step that isn't Work or isn't reached yet, accepting an Open Pull Request before it merges), and when the plan no longer fits it asks the planner (`ws mission replan`), once at a step, and after that you. `ws mission show` (and `ws show`) lists Z and the steps; `ws mission plan` prints it; the mission's page shows it under the brief, and its steps in the details (*Plan*), marked *yours* at a checkpoint and *ws opens it* at an Open Pull Request.
- **Revising.** The planner comes back to the plan in the conversation the mission was briefed in (or one ws opens for it, on the mission's *Talk* tab), on briefing's agent and model: at a step of its own, where ws asks it (*The mission has reached step 3, yours: Plan the fix. Write the steps after it…*); when the orchestrator says the route doesn't fit what's happened (`ws mission replan "<what doesn't fit>"`, once at a step); and when you ask (in its conversation, which is beside the plan on the mission's *Brief* tab, `ws mission replan "<what to change>"`, or confirming a replan a mission's conversation proposed). It reads where the mission stands and what the steps before made, and writes the whole plan again with `ws mission plan --propose --set -`, which ws reads strictly and against how far the mission's got. **The steps behind the mission stay as they were**: those passed, and the one it's at once crew have begun it (or the planner's own): a revision that changes one is refused, saying which, yours too. **What's yours to confirm**: a revision that removes or moves a checkpoint (adding steps before one doesn't move it), or changes Z, comes to you as a proposal in the planner's conversation, saying why it's yours, shown as what changes; the headline says so (*The planner proposes a revision of the plan, in t2: It removes the checkpoint at step 4…*). **Anything else applies** at once, and the log says what changed (*revised the plan, in t2: step 4 added (Fix it, crew, in `api/fix`); step 4 is step 5 now (Review the fix)*). Written at the planner's own step (or accepted by you, if it was yours to confirm), it passes that step and ws moves on; written elsewhere, the orchestrator is run to look at the plan as it is now, once it applies: straight away, or when you accept it if it was yours to confirm. A step in a capsule the mission hasn't got is lifted for it. The planner may decide the rest is other missions, proposing each (raised from this one) for you to confirm. While it's at work the headline is the agents' (*The planner is at work on the plan, in t2*); if it replies at its step without writing the steps after it (it asked something), the step is yours to answer it, or write them yourself and go on. You editing the plan yourself applies as you write it, the steps behind the mission held as they were.
- **Pull requests are opened by a step.** An Open Pull Request step's is opened by ws (above). In a plan written before kinds of step, a crew step whose `makes:` says a pull request opens it: its crew commit, then run `ws mission open-pr`, which pushes the capsule's branch and opens the pull request onto the default branch, holding ws's guarantees about pull requests (see *Guarantees*); the orchestrator can't accept that step until one is open (or merged). Crew at the plan's other steps commit and report done, and don't push or open one; the orchestrator never does. You can open one whenever you like: `ws mission open-pr`, or *Open pull request* on a capsule in the mission's *Work* tab, once GitHub has said it has none. It's refused in a repo with no pull requests (land those), with changes not committed or nothing to open, or with one open from the branch already. A mission without a plan runs as before: its crew open their pull requests themselves (or with `ws mission open-pr`).
- **The PR writer** writes the title and description of a pull request ws opens with none given: ws's own at an Open Pull Request step, crew's (`ws mission open-pr` with no `--title`; crew are told to give none), and yours, from *Write it* in *Open pull request* (it fills the title and description in for you to change), or *Write and open it* with the title left empty. With `--title`, it's opened as given. It writes for the task the pull request is for, not the whole mission: the plan's Work steps in that capsule since the last step that made a pull request from it, up to the one opening it (with no plan, the brief's goal), with each step's checks, what its crew handed off, what you've decided, the capsule's commits and diff, and the repo's own pull request template, if it has one. How the workspace's pull requests are written is `PR.md` beside `ws.toml` (or where `[agents] pr` says), in prose; it stands over the rest. It's an agent run once, headless and read-only in the capsule (it may read the code, `git log` and `git diff`, and `ws show`), told to write for someone who knows the repo, not the mission. ws first finds the pull request can be opened (every refusal above, `review_before_pr` among them), then the daemon has it written, as that takes a while, and opens it as it was asked for: the mission is *Writing a pull request* until then, the orchestrator can't accept the step, and opening it again is refused. A writer that can't write it leaves it titled as the mission is, and the log says so; one that can't be opened once written (GitHub refuses it, say) is put to you, or at an Open Pull Request step, as the step puts it. It runs on `[pr_writer]`'s harness and model (in `ws.toml`, `ws.local.toml` or Settings), else conversations'. Its latest run is kept beside the mission (`pr-writer.log`). `[pr_writer] enabled = false` turns it off: a pull request opened with no title is titled as the mission is, described from its crew's handoff, and opened at once, with no agent and no wait.

**Endings.** A mission ends at Z, or earlier with a reason, in one of four ways: **landed** (at Z, its code merged or landed), **answered** (a finding and no code, at Z or stopping early: "not a bug"), **handed on** (its work raised as other missions, which take it on) or **dropped**. Z is checked when it ends, each part met or not, and how: the plan's Z, or for a mission without a plan, the items of its brief's `## Acceptance`. The orchestrator (or crew) suggests how it ends with `ws mission conclude "<what it came to>" --as answered --met "1: told in the thread" --unmet "2: no bug, so no test"`; ws doesn't judge a part, but refuses a conclusion that doesn't check each one by its number (dropping needn't), landed with a part unmet or its work not merged, and handed on to no mission. Ending it is yours: *End it* on the wrap-up (or `ws mission end`) ends it as suggested; *Say how it ends…*, *End* beside its state, and *Wrap it up* once every step of the plan is done open a form of the ending, what it came to and each part of Z (or `ws mission end --as dropped "<why>"`); *Not yet* puts a suggestion aside. An ended mission is in the past, out of what needs you, its page showing how it ended with Z as checked; *Archive* puts it out of sight, and *Reopen* (or `ws mission reopen`) takes the ending back. Ending it is refused while crew are still on it. **A capsule can end alone**, so it no longer holds the mission open: `ws mission hand-on <repo>/<capsule> "<what for>"` raises a mission for it (or `--to` names one there is), which takes the capsule; `ws mission drop <repo>/<capsule> "<why>"` drops it, keeping its worktree in no mission. Handing on is yours and the orchestrator's, dropping yours alone; both are on each capsule in the mission's *Work* tab, and the mission lists the capsules that ended alone. Ending the mission handed on names where its capsules went.

**Stages.** A brief can list its work as stages for separate crew in turn, as a numbered list under `## Stages`. Briefing agents write them so, naming each stage's capsule as `repo/name` in backticks and putting "Check in with me." on any stage you want to review before the next. The brief stays yours; ws keeps each stage's progress: the crew that worked on it, when it started and when it was accepted. Crew launched or dispatched work on the current stage (the first not accepted), or the one `dispatch --stage n` names, and are told which. When a stage's work is complete, the orchestrator (or you) runs `ws mission accept "<why>"`, refused while that stage's crew are still working or asking, and the next stage becomes current. A record survives rewording a stage: it matches by text, or by position while the number of stages is unchanged. A brief without stages is one stage, the whole brief. The mission's Summary leads with the stages (`✓` accepted, `▸` current with its crew), and `ws mission show` lists them. `ws mission brief --set` and `ws mission ready` warn when a stage names a capsule the mission doesn't have. Once a stage is accepted, ws moves the plan on itself: it starts the next stage's crew in the capsule the stage names (or the mission's only one), dispatched with the stage's text, counting toward the dispatch limit (queued for you with `dispatch = "ask"`). If the accepted stage says to check in, or ws has started as many crew without you as the orchestrator may, it asks you first: *Stage 1 is done (c2: …). Go on to stage 2: …?*, answered *Go on* (ws starts it) or *Not yet* (it waits; launching crew starts it later), without running the orchestrator; any other answer goes to the orchestrator. A stage that names no capsule of the mission's, in a mission with several, isn't started: ws says so, and the orchestrator (or you) dispatches it. With the orchestrator off, crew reporting their stage done puts it in your court: the headline asks you to accept it, and *Accept* does.

**Handoffs.** Work crew finish with a summary and a handoff for whoever comes next, in markdown with four sections: **Done** (what they did and where it is), **Learned** (where things live, what they tried that didn't work, surprises), **Left** (what's still to do) and **Read first** (files, references or URLs). `ws mission done` refuses without one, or with a section missing, or with a *Read first* entry ws can't find (any section can say "Nothing."). It's kept with the mission at `mission:handoffs/<crew>.md` and recorded as a `handoff` artifact; the log and the mission's Summary say what's left, and the Log tab links to it. Crew launched or dispatched next get, ahead of the brief, the latest handoff from their capsule (or else the mission's latest) with where to read what it names, and where the other capsules' latest handoffs are. The orchestrator reads the handoff in full, with less of the crew's terminal output. Briefing crew finish with `ws mission ready` and crew answering the orchestrator's question with `ws mission answer`; neither leaves one.

The orchestrator may dispatch three crew in a row without you; after that `dispatch` refuses and it has to flag you. Anything you do that changes the mission (launching crew, answering or dismissing its question, accepting a stage, editing its brief or title, attaching or detaching a capsule, parking or archiving it), on its page or from a shell, starts the count again, and dispatches that fail to launch don't count. A dispatch that would launch at once refuses a capsule other crew are still working in (two agents in one worktree edit the same files). Crew that have reported done don't count: they've handed their work back, so a dispatch into their capsule ends their still-open session (logged, and resumable) and starts the new crew there. That's what lets a staged mission in one capsule move on by itself. Only one dispatch is queued at a time: a second is refused, and the orchestrator flags you instead. *Dismiss* (or `ws mission flag --clear`) dismisses the flag first, then the queued crew, and says so when there's neither. It may look around read-only (`git diff`, `gh pr checks`) and lift capsules, but doesn't change code or push. Each crew event is handled once. It runs in the daemon, one run at a time per mission (an event that comes while one runs is looked at once it's done), and its output goes to `.ws/missions/<slug>/orchestrator.log`. Each run is kept in `orchestrator/<id>.json` there (the last 50), with the prompt it was given beside it (`<id>.prompt.md`): what it looked at, its prompt, what it said, whether it acted (anything it did through ws is logged with its run's id), and, for Claude Code, every call it made, refused ones marked. `ws mission orchestrate` runs it by hand (`--crew c2` to say what prompted it, `--print-prompt` to see what it would be told). Crew output is saved to `crew-<id>.log` in the mission's folder when a run ends, so it can read it later.

Configure it in `ws.toml` or `ws.local.toml`:

```toml
[orchestrator]
harness = "claude"                  # which agent runs it: see Agents below
model = "haiku"                     # default: claude-haiku-5-5-medium on cursor, haiku on claude, your default on pi and opencode
enabled = true                      # false turns the automatic runs off
dispatch = "auto"                   # "ask" queues dispatched crew, for you to launch or dismiss on the mission's page
max_dispatches = 3                  # in a row without you
```

**Agents.** Four roles run agents, and each picks its harness and model in its own section of `ws.toml` or `ws.local.toml` (local settings win, field by field):

```toml
[crew]                 # agents doing the work
harness = "claude"     # cursor (default), claude, pi, shell, codex, cursor-acp or opencode
model = "opus"         # passed as --model, in that agent's own naming; default: the agent's
allowed = ["claude:sonnet", "cursor:gpt-5.5-high"]  # others Launch crew may start crew on; default: none
trust_folders = true   # ws.local.toml only: answer Claude Code's trust dialog yes for crew; default: false

[briefing]             # agents turning a request into a ready mission
harness = "cursor"     # default: crew's harness (and model)
model = "claude-opus-5-5-high"

[orchestrator]         # the headless agent deciding what's next
harness = "claude"     # cursor, cursor-acp, claude, pi, codex or opencode (it must run headless); default: crew's, if it can

[conversation]         # the agent answering threads, conversations, cells and passes
harness = "claude"     # cursor, cursor-acp, claude, pi, codex or opencode; default: crew's, if it can hold one, else claude

[judge]                # the agent judging a Work step's checks in words, read-only
harness = "claude"     # as [conversation]; default: briefing's agent and model
model = "opus"

[pr_writer]            # the agent writing the pull requests ws opens, read-only
harness = "claude"     # as [conversation]; default: conversations' agent and model
model = "sonnet"
# enabled = false      # no agent: ws titles them as the mission is, at once
```

- A model belongs to the harness it's configured with. A `[briefing]` model with no harness applies to crew's harness.
- `[crew] allowed` lists other agents and models you may launch crew on from a mission's page: each `harness:model`, or a harness alone (on `[crew] model` if it's crew's harness, else on the agent's own model). The model is what follows the first colon, so `pi:sonnet:high` is pi on `sonnet:high`. With any listed, *Launch crew* is a split button: a click launches crew on `[crew]`'s own harness and model, which is always allowed, and its menu launches them on one of the others. The server refuses a launch on anything else. A model picked so is recorded on the crew, and `ws show` and the web UI say what they run on ("claude on sonnet"). Crew over ACP (Codex, `cursor-acp`, OpenCode) resume on it; Claude Code, Cursor and pi resume their session as before, without naming a model. It's a guarantee: crew run only on what it lists, whoever starts them. A plan's step names one in its `on:` (`claude:opus`, or a model alone, on crew's harness), as the process prefers ("bumps on the cheapest model"), and a plan naming anything else is refused, with the list; the planner and the orchestrator are told it. The orchestrator picks one with `ws mission dispatch --on` or `ws mission repeat --on` ("if a fix fails twice, try a stronger one"), refused likewise. A step of a plan written before ws checked, naming one that isn't allowed, runs on crew's own, saying so. Resumed crew run on what they ran on. `ws.local.toml`'s `allowed` replaces `ws.toml`'s whole (`allowed = []` there allows nothing else), and `ws` refuses an entry with no harness.
- `WS_DECK_HARNESS` overrides `[crew] harness` for one session. `WS_CURSOR_AGENT`, `WS_CLAUDE_BIN` and `WS_PI_BIN` name the binaries if they aren't `cursor-agent`, `claude` and `pi`; `WS_CODEX_ACP` names Codex's adapter if it isn't `codex-acp` on your `PATH` (a path, or a command line such as `npx -y @agentclientprotocol/codex-acp`), and `WS_OPENCODE_BIN` the OpenCode binary if it isn't `opencode`.
- **Cursor.** Crew launch with `--trust`, so Cursor's workspace-trust dialog doesn't swallow the opening prompt, which is typed in once its screen settles. The orchestrator runs with `--force`: Cursor's print mode can't limit which commands it runs, so it relies on being told to act only through `ws`. A model your Cursor plan doesn't include fails the run ("Model unavailable on Free"); `auto` works on every plan.
- **Claude Code.** Crew get the opening prompt as their first argument and a session id ws chose (`--session-id`), so they can be resumed. Claude Code asks once per repo whether to trust it: the first crew in each repo opens on its trust dialog. No hook runs until it's answered, so ws watches a crew's screen as it starts: on the dialog, the crew needs your permission ("Claude Code asks whether to trust …", under *Needs you*). Answer it in its terminal, and the prompt runs and the crew's working; that repo's later capsules don't ask again. Crew the orchestrator dispatches into a repo nobody has trusted yet wait at that dialog for you, so answer it once per repo when you start using Claude Code. In a throwaway workspace, where every repo is new and an agent may be the one driving, `[crew] trust_folders = true` in `ws.local.toml` has ws answer it yes (`just test-workspace` and `just try` set it). It's read from `ws.local.toml` alone, as trusting a folder lets its agent run what's in it. The orchestrator runs with `--permission-mode dontAsk` and an allowlist: `ws mission`, `ws lift`, `ws status`, read-only `git` (`diff`, `log`, `show`, `status`, in the mission's capsules and every repo's `.ground`) and `gh pr view/checks/list`, and reading files. Anything else is refused without a prompt.
- **pi.** Crew get the opening prompt as their first argument and a session id ws chose (`--session-id`), so they can be resumed (`--session`). Models are in pi's naming (`anthropic/claude-opus-5-5`, `sonnet:high`), from whichever providers you've logged in to. pi doesn't ask permission for tools, but in a folder with project-local pi resources (`.pi/settings.json`, `.pi/extensions` and the like) it asks whether to trust them before the prompt runs; ws leaves that to you, since trusting a project runs the extensions it ships. The orchestrator gets `read`, `bash`, `grep`, `find` and `ls` but not `edit` or `write`. As with Cursor, nothing limits which commands it runs, so it relies on being told to act only through `ws`. With no `[orchestrator] model`, it runs on your pi default, so name a small model there if that's a large one.
- **Codex.** OpenAI's Codex runs the headless roles (the orchestrator, conversations, threads, cells, passes and the Team page), and crew (see *Crew over ACP* below). ws speaks to it over the Agent Client Protocol, through its adapter, `codex-acp` (`npm install -g @agentclientprotocol/codex-acp`, which brings a Codex of its own; sign in as you would for Codex). Each run is held in the daemon for its one turn: it starts the adapter, runs the turn and writes what happened as it goes, which the run's page reads. Codex runs in its read-only sandbox, without network, and is told to ask to run outside it the commands that need to (`ws mission …`, `ws lift`, `gh`). ws answers each ask itself: one of the commands the role may run (the same lists as Claude Code's), run alone, is let out; anything else (another command, one chained to more, an edit) is refused, and Codex is told to carry on without it. Inside the sandbox it reads what it likes and changes nothing. A conversation's turns go on in one Codex session, which `codex resume <id>` also opens. Models are Codex's, with their reasoning effort in brackets: `gpt-6-luna[low]` for the orchestrator and `gpt-6.1-sol[medium]` for conversations unless `model` says otherwise; `gpt-6.1-sol` alone takes your Codex default effort.
- **cursor-acp.** The Cursor CLI spoken to over the Agent Client Protocol (`cursor-agent acp`), as Codex is, rather than in its print mode: an alternative to `cursor` for the headless roles, and for crew (see *Crew over ACP* below), in Cursor's agent mode. Each run is held in the daemon for its one turn, as Codex's is. It works without `cursor`: set every role to `cursor-acp` and conversations brief and plan, and the orchestrator dispatches, with the same `ws` commands Claude Code's do. Every role runs in Cursor's agent mode: its ask and plan modes run no command, or only in a read-only sandbox, so they can't run `ws … --propose`. Cursor writes files in its session's folder without asking, so a conversation, the judge or the orchestrator runs from a folder of ws's own (`.ws/converse/cursor-acp`), and is told the folder it works on; it reads anything, and a write anywhere else is asked of ws, which refuses it. It signs in with what `agent login` left. Every command Cursor's own allowlists (`~/.cursor/cli-config.json`, a repo's `.cursor/cli.json`) don't already allow is asked of ws, which lets out the role's commands (the same lists as Claude Code's), run alone, and refuses anything else; Cursor goes on without it. What your own allowlists allow (a command, a `Write(…)` rule) runs without ws asked, as it does for any Cursor run. Its questions are answered skipped (it's told to ask in its reply, or decide and say what it assumed), as nobody's there to answer. Models are named as for `cursor`, but over ACP Cursor offers each model once, at its own effort, so an effort on the end of a name is dropped (`claude-haiku-5-5-medium` runs `claude-haiku-5-5`); `auto` works on every plan. Choosing a model also makes it Cursor's default for later ACP sessions. Its sessions are kept in `~/.cursor/acp-sessions`, apart from the CLI's chats, so `cursor-agent --resume` can't open one.
- **OpenCode.** OpenCode (opencode.ai) spoken to over the Agent Client Protocol (`opencode acp`), as Codex is: crew (see *Crew over ACP* below) and the headless roles. Sign in to your providers as you would for OpenCode (`opencode auth login`); with none, it runs on OpenCode's own free models. Crew work in OpenCode's `build` agent, as your OpenCode config has it: by default OpenCode asks for nothing but reading outside the capsule, reading a `.env` file and a call repeated three times over, so crew run what they like unless your config's `permission` says `ask`. What it does ask waits for you on the crew's card, but for crew's own commands, which ws lets out. A headless run (the orchestrator, conversations, threads, cells, passes and the Team page) is held in the daemon for its one turn, in an agent of ws's own, `ws-headless`, given to OpenCode for the run in `OPENCODE_CONFIG_CONTENT` (which replaces one of yours, for that run). Its rules come after your config's, so they hold whatever yours say: it may read and search, anywhere but a `.env` file; every shell command is asked of ws, which lets out the role's commands (the same lists as Claude Code's), run alone, and refuses anything else, and OpenCode goes on without it; editing, fetching and subagents are off. Models are OpenCode's, `provider/model` (`opencode models` lists yours), with one of the model's variants in brackets for its effort: `anthropic/claude-sonnet-5-5[high]`. With no `model`, every role runs on your OpenCode default, so name a small model under `[orchestrator]` if that's a large one. A conversation's turns go on in one OpenCode session; one OpenCode no longer has (deleted, or `OPENCODE_DB=:memory:`) starts afresh. OpenCode treats a message that starts with `/` as one of its commands, and drops one it doesn't know.
- **Crew over ACP.** Codex, `cursor-acp` and OpenCode crew have no terminal. The daemon holds each as an agent it speaks to, and the web UI is how you see and talk to them: their crew page is their conversation alone, kept by ws as it happens (`crew-<id>.acp.jsonl` beside the mission), with no *Terminal* tab. A message from the page is sent as one; one sent mid-turn waits for the turn to end. *Interrupt* cancels the turn. A permission or a question waits on its card, answered with the agent's own options: no screen to read, no keys to guess. Crew run `ws` commands, `git add`, `git commit`, `git push`, `git fetch` and `gh pr` without asking you, on their own or chained (`git add main.go && git commit -m …`, `ws mission done … --handoff - <<'EOF'`); anything else they ask for waits for you. Codex crew work in their capsule in Codex's workspace sandbox: they change and commit files there, and anything else (Go's build cache, the network, a package to install) is asked for. Ended crew resume from their page with your message, on their session, at the role's model. `codex resume <id>` opens a Codex crew's session in Codex's own terminal.
- Crew and the orchestrator don't inherit the session identity of an agent ws itself runs in (`CLAUDECODE`, `CLAUDE_CODE_SESSION_ID`, pi's `PI_SESSION_ID` and `PI_MODEL`, OpenCode's `OPENCODE` and `OPENCODE_PID`, and the like), so ws run from Claude Code's, pi's or OpenCode's terminal launches independent sessions.

**Artifacts.** An artifact points at something the mission produced or relies on, so the next crew, the orchestrator and you can find it. `ws mission artifact add <what> [--kind <kind>] ["label"]` records one, and crew are taught to record what they make. `<what>` can be:

| You give | It records | Where it is now |
|---|---|---|
| A file in one of the mission's capsules (path or `repo/capsule:path`) | `backend/cache-lookup:docs/proposal.md` | `draft`, `in review` (its capsule's PR is open), then `landed`: once the PR merges, or the capsule is burned and the file is on the default branch, it's read from `.ground` |
| A file on a repo's default branch (a path in `.ground`, or `repo:path`) | `records:adr/0042-cache.md` | `landed`, or `missing` |
| A file in the mission's private folder, `.ws/missions/<slug>/files/` (path or `mission:path`) | `mission:spike.md` | `private`: only on your machine |
| A PR or issue (`repo#N`, or its GitHub URL) | `backend#88` | `open` or `merged` when it's a capsule's PR |
| Any other URL | itself | `link` |

Kinds say what it is: `proposal`, `adr`, `note`, `doc`, `plan`, `handoff`, `pr`, `issue`, `link`, or any lowercase word (default `pr`, `link` or `doc`). Adding the same thing again updates its kind and label; `ws mission artifact rm <ref>` forgets one. A path outside those places is refused, with what to do instead. `ws mission show -f llm` lists every artifact with where to read it, the orchestrator sees the same list, and `ws mission dispatch --read <ref>` (repeatable) gives the next crew a *Read these first* list with what it's dispatched to do. The next stage may build on an artifact still in review; a brief that wants to wait for one to land says so as a check-in.

**Proof.** An image or a video (`.png`, `.jpg`, `.gif`, `.webp`, `.svg`, `.avif`; `.mp4`, `.webm`, `.mov`) recorded as an artifact is shown as it is when you open it from the mission's page: an image whole (click it for full size), a video to play. It isn't read as text, so it can't be edited or commented on there. One recorded with `--kind proof` and a label saying what it shows is crew's proof of something they checked by seeing it. A Review step's task offers to *See* an image and *Watch* a video, as it offers to *Read* a document. Crew are now taught to hand what they saw over as a verification bundle, below, where it belongs to the check it shows.

**Verification bundles.** A bundle is what crew hand over to show that their work is as its checks say: each check with what it came to (`met`, `unmet` or `can't tell`) and what shows it, how you can try it yourself, what wasn't verified, and what its crew noticed beside the checks. Crew write it whole, in markdown, for the step they're on, once their work is committed:

```
ws mission bundle --set - <<'EOF'
## Summary
Check 1 is met. Check 3 could not be tested, because only one tab could
be driven.

## Try it
- try: `npm install && cd web && npm run dev`
- look for: Appearance, on http://localhost:5173/settings
- look for: the row of swatches under Accent

## Checks
1. met: Picking teal and then rose changed the tabs, buttons and links immediately.
   - shows: `pick.webm` Picking three colours in a row
     - made by: `npm run shots -- accent-picker --out pick.webm`
   - shows: `settings-light.png` Light, after picking teal
     - before: `settings-light-ground.png`
   - lines: `web/src/settings/Appearance.tsx:41-88`
3. can't tell: Only one tab could be driven, so a change in a second tab was never observed.
   - try: open Settings in two tabs, pick in one, look at the other
   - ask: Add a test for the storage event

## Not verified
- Safari: it is not installed here.

## Noticed
- At phone width, the names of the swatches are cut off.
EOF
```

- **Read strictly,** as a plan is: `ws mission bundle --format` says how one is written (and, to crew, their step's checks by number), and one that isn't right is refused with every problem and its line. Where the step has checks, each one in words is there once, by its number in the step; a check that's a command is ws's to run, and is left out. Where the step has none, the checks are the bundle's own, numbered in order. A `met` needs something that shows it: a `shows:` or the `lines:` it's done in.
- **Held to a length,** so that what you have to decide isn't lost in what was measured. Its summary says how it came out in three sentences at most (80 words): what's unmet and what's wrong with it, or that every check is met, then what's left for you. A check that's met is a sentence or two (50 words); one that's unmet, can't tell or yours, 100; an item of *Not verified* or *Noticed*, 50. Counts, timings and everything that was tried go in a text file the check `shows:`, and a thing is said once. One well over (more than a fifth) is refused, saying where the rest goes.
- **Written to be read by someone who wasn't there:** the most important thing first (the outcome, then each check's finding), in short, plain sentences with one fact in each, exact names and numbers, everyday words and no idioms. A check's first sentence says its finding by itself, in the words of the work's user; an unmet check shows one example of what's on screen; why it happens and how to put it right go in its `ask:`. Facts of the same kind (a result at each size, in each browser) are `- point:` lines under the check, shown as a list. `ws mission bundle --format` tells crew so, and its example is written that way. `scripts/bundle-replay.sh` replays the session of crew who handed a bundle over, to see what they'd write as this build tells them to, and `scripts/bundle-skim.sh` asks a fresh reader what they take from a bundle's summary and the first sentence of each check.
- **`## Noticed` is optional:** what crew came across that isn't one of the checks (something off beside the work, or older than it), a line each. It sends nothing back.
- **A `try:` is one command,** in backticks with nothing after it, so you copy it once: setup, `cd` and what starts it are chained with `&&` (`a && cd x && b`). What to do or see in the UI once it's running is in `look for:` items, one each; a `try:` with nothing to run is in words. One with a second command, or words after its command, is refused, saying so.
- **What shows a check** is a file: an image, a video, or text such as what a command printed (1 MB of text at most, 50 MB of image or video). It's named by its path from where crew stand, or in the mission's private files folder. Under a `shows:`, `before:` is the same on the default branch, shown beside it, and `made by:` the command that made it.
- **Its files are copied** into a folder of the bundle's own, `.ws/missions/<slug>/bundles/b1/`, so it's whole whatever becomes of the files it was made from. Its record is the mission's, in the workspace's records.
- **It's of a commit:** the one its capsule is at. It's refused while the capsule has changes that aren't committed, and it says so once the capsule has moved on (*Stale*).
- **It's never edited.** A later bundle of the same step and capsule takes its place, and each says which.
- **Its ref is `b1`,** numbered in its mission: `ws show b1` has it, with its step, its crew and its capsule; `ws mission bundle` lists the mission's.
- **It's yours to approve or send back,** as a review is, and yours alone: crew and the orchestrator are refused. *Approve* (`ws mission bundle approve b1`) says what it shows is as its checks say. *Not right* on a check says what's wrong of it, and *Send back* (`ws mission bundle send-back b1 --check "3: it's gone after a reload"`, once for each check, with words after the ref for the bundle as a whole) records what you said on the bundle and tells the crew who made the work, with each check and what the bundle showed for it: typed in if they're running, resumed if they've ended, else fresh crew in their capsule. The bundle they hand over next takes its place. *Looked at* on a check keeps your place, as *Viewed* does on a file in a review: approving doesn't need it.
- **A Review step looks at the bundles** of the steps before it, back to your last: each is what's to approve, first, with the capsule's review beside it. Approving every bundle it looks at passes the step, as going on does; your review of the capsule alone doesn't pass a step that looks at a bundle too. The step's headline goes to the bundle (*Verify b1*, `v`), and the tab's count is the bundles waiting for you.
- **So do a step's checks, when they're yours to say** (the judge can't tell, or they've come round unmet as often as ws lets them): what ws asks names the bundle crew handed over for that step, and its headline goes to it (*Verify b1*, `v`), beside *Pass it* and *Send it back*. Approving it passes the step, as passing it as it is does. At a Work step, saying what's not right of it sends the step back with what you said, as *Send it back* does with what's unmet: its crew put it right, hand over another, and report done, and ws runs the checks again.
- **Where it shows:** the mission's *Verify* tab. The bundles that are yours to approve are in a row at its top, what a step waits on first, each with how many of its checks are left for you; the rest (sent back, approved, and stale: replaced, or of a capsule that's moved on) are behind the button at the row's end. The bundle that's open is in two columns, which fill the tab and scroll by themselves (on a narrow window they stack, and the page scrolls). The first has who made it and of what, its summary (a few lines, until you ask for *More*), what it didn't verify, what its crew noticed and how to try it (folded), and then its checks: *Left for you* first (what isn't met, and what the judge didn't find so), then *Settled without you*, a line each, with the step's commands as ws ran them at the bundle's commit. The second is the check that's open, with what it found (its points as a list), what the judge said of it at that commit and what shows it: images and videos as they are (an image beside its `before:`), text as a transcript. `j`/`k` move between checks. A check left for you is yours to approve (`y`, *Approve check*): it leaves *Left for you* for *You approved*, folded under it, and marking it again puts it back; `y` on a settled check marks it as looked at, to keep your place. `n` says a check isn't right. Under *Left for you* is the bundle's approval (`mod+⏎`), or its sending back when you've said something isn't right; it approves at any time, and is the button to press once nothing's left for you. Approving the last check left for you says you're all clear, with the approval again; approving or sending back a bundle goes on to the next that's yours, and with none left the tab says so, with the bundles a menu away (one opened from there closes with `esc`).

**Verify steps.** A plan's *Verify* step has work shown to be right by someone who didn't do it:

```
4. Work: **Accent picker.** Add the picker to Settings.
   - in: `web`
   - makes: the picker, its shots script, and a decision saying where the script is and how it's run
5. Verify: **Browser check.** The picker, running, shown with the shots script.
   - in: `web`
   - check: a pick applies at once, light and dark
   - check: `npm test` passes
   - yours: it reads well at phone width
6. Review: **Approve the browser check.** The bundle from step 5.
```

- **Its checks** are what's to be shown: in words, which its verifier settles; a command, which ws runs; or `- yours:`, one the verifier doesn't settle, a matter of taste or judgement kept for you, which it leaves in the bundle as `yours:` with what helps you settle it. It takes `in`, `on`, `limit` and `only if` besides, and needs at least one check.
- **ws starts a verifier** when the mission reaches it: fresh crew in the capsule the work is in, on the step's `on:` or crew's own. They're told their step, its checks and whose each is, who made the work and where their handoff is, and to start from `ws show` (the mission's decisions say where a script is and how it's run). They aren't given the process: the planner copies what it says of verifying into the step.
- **A verifier changes nothing.** ws refuses its bundle if the capsule's files have changed, or it has moved on from the commit it was at when the verifier started.
- **Its bundle is its done:** handing it over ends the verifier's part (`ws mission done` is refused: there's no handoff). ws then runs the step's commands in the capsule and takes the rest from the bundle.
- **Nothing unmet, and ws passes the step.** What the verifier couldn't tell, and what's yours, is left for you in the bundle, which a Review after it asks you to approve.
- **Something unmet, and the work goes back** to the crew who made it, with what's unmet and what the verifier said: typed in if they're running, resumed if they've ended, else fresh crew in their capsule. When they report done, ws starts a fresh verifier, told what went back, and its bundle takes the last one's place. After `[missions] max_rounds` rounds (or with no crew to send it to), the step is yours: *Pass it* as it is, or *Send it back*.
- **It isn't the orchestrator's to accept,** nor yours to accept from its crew's done: ws settles it on the bundle.

Running a bundle's commands from the page isn't built yet.

If your workspace keeps proposals, ADRs and notes in a repo of their own, name it in `ws.toml`, and crew and briefing agents are told to lift a capsule there when a stage produces one (its README or AGENTS.md keeps its conventions):

```toml
[missions]
records = "records"
```

**Finding your way: `ws show`.** Everything in a mission has a ref: a stage (`s2`), crew (`c1`), a capsule (`repo/name`), an artifact (by its ref, or the path of its file), a review (`r1`) and its comments (`r1.c2`), a thread on a document (`t3`), a decision (`d1`), a verification bundle (`b1`), an orchestrator run (by its id). `ws show <ref>` shows one (an address works too: `ws:<name>`, or one of the workspace's on GitHub, as a link in a document has it): what it is, the essentials, and what it links to, each as a ref to show next. Crew show which stage they worked on, their dispatch and handoff, and what they recorded; a document shows its history, the stage it was made for, who recorded it, its threads and the handoffs that mention it; a stage shows its crew and what was made for it. `ws show` alone is the mission (`ws mission show`), which lists its crew, reviews, open threads and decisions too. A file (`repo/path`, `repo:path`, `:12-20` for some lines), from anywhere, says what's been said of it in any mission: comments on it (open ones first), decisions and threads about it, and what names it (talk, a mission's brief or a handoff, a decision, a review's comment, a pull request's words, a document in a space), each with the mission it's in; crew are told to look before they change one. What a text names is found as it's written, by its name (`demo/main.go:12`, `demo#12`, `RAM-123`) or by where it is (a pull request's address, a plugin's thing's); a repo named by its name alone isn't linked, being a word as often as a name. `ws doctor --reindex` finds it all again, which is what links what was written before a plugin named its things. Any view says what points at it from beyond its mission. Views are short and say what they left out; `--full` shows it all. `-f llm` is for agents: crew, the orchestrator and crew answering a question are told to start from it rather than having everything pasted into their prompt. `scripts/coldstart.sh` checks that's enough: it asks a fresh agent a question about a ref, with nothing but `ws show` to go on, and prints its answer and what it read.

**Decisions.** What was decided and why: `ws mission decide "<what>" --why "<why>" [--about <ref>]... [--from <ref>]`. Crew record the decisions they make within their part (an approach taken, an option ruled out), the orchestrator those about the mission's course; your answer to the orchestrator's question is recorded as one, with the question (not a check-in's *Go on* or *Not yet*). A decision is never edited: a new one replaces it (`--supersedes d1`), or it's withdrawn (`--withdraw d1 "why"`). Those that stand are shown wherever they matter: in `ws show` of what they're about, the mission's view and its *Summary* and *Log* in the web UI, and the orchestrator's prompt; crew are told not to go against one without asking you. They're kept in the workspace's records (`.ws/ws.db`), and logged.

**Files.** A mission's record, its log, its decisions, its reviews and its threads and conversations are in the workspace's records (`.ws/ws.db`): `ws mission show`, `ws show <ref> --mission <slug>` and the web UI read them. What's written to be read is in `.ws/missions/<slug>/` at the workspace root (`brief.md`, the private `files/` folder with its reviews rendered for crew, its bundles' files in `bundles/`, and crew output). They are local to your machine. `ws` adds `.ws/` to the config repo's `.git/info/exclude` so they never show up in its `git status`, and `ws doctor` checks this. `ws export <mission> [dir]` writes a mission out as a folder of files, to read without ws or hand to someone: its folder as it is, and beside it its record, log, decisions, conversations and reviews as the files they were once kept in (`mission.toml`, `log.jsonl`, `decisions.jsonl`, `conversations/t1.jsonl`, `reviews/r1.json`).

**What ran.** `.ws/exec.log` records every process ws starts for missions, and what it sends them:

- each crew's command line (ready to paste into a shell), the folder it ran in, and its mission environment (`WS_*`, `TERM`, the first entry on `PATH`; never anything else, which may hold tokens);
- the chat id Cursor gave it, or why it gave none;
- the prompt typed into it, and how it ended;
- each orchestrator run: the prompt file it wrote, the exact agent command, how long it took, and how it ended;
- automatic runs that were skipped, and why;
- the daemon starting.

Every ws process appends to the same file, so `tail -f .ws/exec.log` follows the whole workspace. It rolls over to `exec.log.1` at 10 MB.

**Web UI.** Where you work: missions, kept live as they change, crew, review, your teammates' pull requests (see *Reviews*), and what everyone's been doing (see *Team*). `ws web` (or `ws` alone) opens it in your browser. It runs in the workspace's daemon (started if it isn't running), which stays up until it's closed (`ws daemon stop`). The workspace's name, at the top of the sidebar, opens its menu: *Settings* (where the theme is, and the way to arrange the sidebar), *Keys* (`?`), and *Shut down the daemon…*, which asks first, then closes it as `ws daemon stop` does, its crew hung up and the page with it. At the menu's foot is which ws this is: its version, as `ws --version` says it of the daemon serving the page (a build between releases as the release it's after, `-dev` and its commit; the whole of it on hover). On a page that wants the room (a mission and what's under it, a pull request's review, a repo's files, an artifact) the sidebar folds to a rail of icons, with a dot on a place that has something for you and each thing that needs you as its icon; it opens over the page while the pointer is on it, or the keyboard's focus is in it.

**A mission's page** leads with its headline: whose court it's in, and what moves it on, and you can act on it there: answer the orchestrator's question (one of its answers, or your own words), ask an agent to answer it, dismiss it or queued crew, accept a stage, end it (as suggested, or as you say, with Z checked), hand a capsule on or drop it, edit the brief, rename it, park, archive, unpark, launch crew (new crew on the current stage, the queued crew, a briefing agent) or resume ended crew; a re-review, a PR to open or merge, and the log are links. Crew start in the daemon, as dispatched crew do.

**The process.** How work goes in the workspace, in prose: the steps work takes, whose turn each is, what done means, and when it's yours ("Bugs are reproduced first, with a failing test, then fixed"; "Once a draft's checks are green, it's mine to review"). It's the team's: `PROCESS.md` in the config repo beside `ws.toml` (or where `[agents] process` says), changed by commit like the rest of the config. The planner reads it as it's written to plan each mission (see *The plan*), and the orchestrator reads it to know what to do when something happens; a plan's step may end with its words in double quotes, which have to be there. `ws process` (and `ws show process`, for an agent) shows it, where it is and whether it's as committed, and so does *Process* in Settings. There's nothing to read back or confirm: what ws holds itself is a guarantee.

**Guarantees.** What ws holds on every mission, where it acts, whatever an agent reads: `[missions]` in `ws.toml` (or yours, in `ws.local.toml`), each off unless it's set. What the process prefers is the planner's and the orchestrator's to apply; a guarantee is ws's, and refuses whoever asks, saying why and what to do.

```toml
[missions]
review_before_pr = true    # no pull request until your review approves the capsule as it is
review_before_land = true  # the same, before ws land of a mission's capsule
review_before_ready = true # a draft isn't marked ready until your review approves it
replies_need_me = false    # crew's replies to people post without waiting for you (default: they wait)
end_when_merged = false    # a mission suggested landed waits for you to end it (default: ws ends it)
max_rounds = 2             # a step's checks, or its pull request's fixes, unmet this many times, and it's yours (default 3)
```

- `review_before_pr`: `ws mission open-pr` (and *Open pull request*) opens a pull request from a mission's capsule only once your latest review of it approves the commit it's at. A checkpoint just before a step that opens one says to review the work there. Crew opening one with `gh` themselves aren't held: that's why a plan's step opens it, through ws.
- `review_before_land`: `ws land` of a mission's capsule, in a repo with no pull requests, only once your latest review of it approves it as it is.
- `review_before_ready`: ws marks a mission's draft pull request ready (at its Open Pull Request step, or `ws mission pr-ready`) only once your latest review of its capsule approves the commit it's at.
- `replies_need_me`: on unless set off. A reply crew draft to a person on a mission's pull request is posted, as you, only once you approve it. A reply to a bot needn't wait.
- `end_when_merged`: on unless set off. Once the orchestrator suggests a mission ends landed (every part of Z met, every pull request merged or capsule landed), and nothing else on it waits (a question or step of yours, crew queued or still at work, a Plan whose build is yours to say), ws ends it, landed, and it's in *Past*: archive it when you like. Held until then, it ends once nothing does. Any other ending, and one crew suggest, waits for you. A merge that passes the plan's last step wakes the orchestrator to suggest how it ends.
- `max_rounds`: always held, 3 unless set. A Work step's checks come round unmet, or an Open Pull Request step's pull request comes back for fixing (red checks, a conflict), at most that many times before it's yours, whatever the process says (see *Checks* and *Looking after a pull request* under *The plan*).

The planner and the orchestrator are told which are on. They're on the Settings page too, under *Missions*, as *Guarantees*.

**The Settings page** (*Settings* in the workspace's menu) is your settings for the workspace, in sections: the list of them is beside the sidebar, and the one you're in beside that (on a narrow window the list runs along the top). Each section has an address (`/settings/agents`), and `/settings` is the first.

- *Appearance*: the theme (light, dark, or as your system has it), the web UI's accent, and the sidebar's arrangement, as a button that puts the sidebar into its edit mode.
- *Notifications*: desktop notifications, on or off (the bell beside *Needs you* in the sidebar is the same switch).
- *Agents*: each role's agent and model (crew, briefing, the orchestrator, conversations, the judge, the Team page) in one table, with why a role can't run if it can't, and what it follows where it isn't set itself (the Team page's agent follows conversations', and the judge briefing's); under it, what else crew may be launched on.
- *Missions*: where a mission's briefed, the orchestrator's switches, and the *Guarantees*.
- *Team page*, and *Repos*: the git protocol, and the team's repos as `ws.toml` has them (what each is called, its aliases and group, and how work lands in it), shown, not changed.
- *Runs*: *Local names*, whether the workspace and its runs have names under `.localhost` (`[router] enabled`) and the name they end in (`[router] name`, the folder's unless you say). Yours alone, as the router listens on your machine; the daemon reads it again within a few seconds.
- *Process*: the workspace's process, as it's written.
- *Plugins*: the workspace's plugins, and adding, updating and removing one (see [Plugins](#plugins)).
- *Set by you*: everything your `ws.local.toml` sets, each with what it would be without yours and *Reset*; the keys either file has that ws doesn't read (the section's name has a dot while there are any); and what this browser keeps.
- *About*: which ws this is; *Updates*, the latest release as the daemon last found it, when it last looked, *Check now*, and whether it looks by itself (`[updates] check`); where the workspace and its files are; and *Shut down the daemon…*.

Most settings are the ones in `ws.local.toml`. Each row says what the setting is and its key in the file, then its control: a switch, a few choices side by side, a list, a number stepped or typed, a list of picks each taken off with its `×`. Under the control is where the value comes from when that isn't ws's default: *the team's* (`ws.toml`), or *yours*, with what yours is over and *Reset*. The theme and desktop notifications are kept in the browser, and say so. A role's agent is picked from the harnesses that can play it, with those that aren't installed marked. Its model is picked from the models that harness lists (`cursor-agent --list-models`, `pi --list-models`, Codex's catalog, Claude Code's aliases), or *Another, by name…* for one typed (a harness that lists none has the field alone); changing the agent takes your model for the old one away. A change is written at once, with *Undo*; *Reset* takes yours away, and the team's or the default applies again. A value the key can't take is refused, with why, and nothing is written. The team's `ws.toml` is shown, not changed, and repo commands (`after_create`, `verify`) and paths stay file only. *Find a setting…* at the top of the list (`/`) reads every section: what's found is shown by section, each row working as it does in its own. Commands, the web UI and runs read the settings each time.

**The orchestrator's page** (*Orchestrator* in the sidebar) says how it's set up (when it runs, its harness and model, whether its dispatches start at once or wait for you, and how many it may start in a row), which missions it's on now, and its latest runs across missions. Each run has a page: what it looked at, how it went (acted, did nothing, failed, a second try), what it said, every call it made with refused ones marked, and the prompt it was given. A mission's sidebar lists its own latest runs.

**Find** (`⌘K`, or *Find…* in the sidebar) gets you to anything by what you remember of it.
- **Type a few letters** of a mission, a page, a conversation, a pull request's title or a file's path. Results come in groups, the best of any kind first, with a card for the one the cursor's on beside them.
- **A name or an address goes straight there:** `demo/main.go:12-20`, `demo#12`, a path on disk, or a link copied from GitHub to a file or a pull request.
- **A file is listed once,** from the default branch, saying which capsules have changed it. What only a capsule has is listed as the capsule's (`demo/token-cache:internal/store.go`). `:12` after a path goes to the line.
- **A pull request** is found by its number (`12`, `#12`), its title, its branch or who it's by (`@sam`), from what the Reviews page last read. Find doesn't ask GitHub, so open Reviews once first.
- **A first character narrows:** `#` pull requests, `/` files (`⌘P` opens Find this way), `>` things to do. `⇥` narrows to the group the cursor's in, and again goes back.
- **`⏎` goes, `⌘⏎` in a new tab, `⌘C` copies the name** of what the cursor's on.
- **`⌘K` on a row acts on it,** wherever it's found: a mission, a conversation, an artifact, a pull request or a plugin's thing lists what can be done to it, as its own page's Find would, and `Esc` steps back. What's done to one thing is only ever under that thing: typing *resolve* finds no conversation's *Resolve*.
- **The places are the sidebar's,** each plugin's views among them, with *Settings* and each of its sections (*Appearance*, *Agents*…) found by name. *Shut down the daemon…* and, when one's out, *Upgrade to ws …* are among what there is to do.
- **With nothing typed** it starts from where you are. On a page of one thing (a mission, a conversation, a page, a pull request, a file, a space's document, a plugin's thing) it shows what can be done to it first (a mission's next step, then park or unpark, stop live crew, archive or unarchive, and its brief; a conversation resolved or reopened, and the mission it became; an artifact's versions compared, or a discarded one restored; a pull request docked here or opened on GitHub; a plugin's thing's actions, *Start a mission about it*), then what needs you, then what's *Linked* to it, each saying how (*what this is about*, *came from this*, *names this*; what a mission's brief and notes name is the mission's), then where you've been lately. Elsewhere, what needs you, where you've been, and the places. The cursor starts below what can be done, so `⌘K` then `⏎` never does it by reflex. Typed, what's linked to where you are comes a little higher.

**Repos** (*Repos* in the sidebar, `g f`, at `/files`) shows any repo's worktrees without an editor open: their files, their runs and their apps.
- **The left column** lists the workspace's repos as ws labels them: display name and colour, by group, with hidden groups folded.
- **A repo opens on its ground,** its default branch as it's checked out, with the file tree and then the file.
- **The title** (*Demo / ground*) switches to any of the repo's capsules (`w`), each with its branch and mission. A file you have open stays open if it's there.
- **Its tabs,** beside the title, are what there is of the worktree: *Files* (the tree and a file), *Runs* (what's running in it, and its recipes, each with *Run*, lasting while the page is open, as one from a mission's page does) and one for each of its apps (a recipe of its `ws.repo.toml` with a `page`: see *Apps*, under Runs). A tab other than *Files* has the room of the tree and the file both. The tab's in the address (`?view=docs`), so a link opens a capsule's docs. A mission's capsule's files (from its *Work* tab) have the same tabs.
- **A capsule's files** read as in its mission's explorer: everything, or only what's changed since it left `origin/<default branch>` (`c`), with changed lines marked. `t` finds a file, and `v` reviews a mission's capsule's changes.
- **A markdown file reads rendered,** as documents are (see *Artifacts*, below): a link to another file opens that file here, in the same tree, at the heading it names; a link to a folder opens its README, or the folder; an image is read from beside the file. `m` switches to its text.
- **Runs** of the ground or capsule open, and its `ws.repo.toml`'s recipes to run, are under the title (see [Runs](#runs)).
- **Edit as a page** (`⏎` on a markdown file that's open) opens it as a page tied to the file, with the page's tools, saved back to the repo when you say (see *A file as a page* under Pages).
- **Only the workspace's repos and their worktrees** can be read this way.

**A crew's page** is a click away (`/missions/<slug>/crew/<id>`, from the mission's crew or a headline that's about crew). Its terminal is its screen, live, and typed into while it has focus, Esc included, so you can answer it or approve what it asks to run from the browser. It's drawn at the crew's size with the font scaled to fit; ⌘C copies a selection and ⌘V pastes into the crew. Enter counts as you talking to it. *Stop* hangs it up. Only crew running in the daemon have one.

**A mission's page** has six tabs (`1`–`6`): *Summary* (where it stands, the goal, stages, crew, capsules, its outputs and what happened lately), *Brief* (the brief and the plan, edited as they read, with its planner's conversation beside them: see *The Brief tab* below), *Work* (what's been made: its capsules and outputs in a list, and the one that's open beside it), *Verify* (its verification bundles: what crew handed over to show their work, each check with what shows it), *Log* (the whole log, and crew that worked in none of its capsules, such as a briefing agent: what each was dispatched for, how it ended, its handoff; crew in a capsule are on *Work*) and *Talk* (the conversations about it). `j`/`k` move between capsules, artifacts and handoffs, or a bundle's checks; `⏎` opens one (a capsule's review, an artifact), `v` reviews a capsule and `o` opens a PR or an artifact's link. PRs come from GitHub after the page has drawn. A mission's outputs are what it recorded (`ws mission artifact add`): its documents, handoffs and reviews, and the artifacts its crew made (`a3`, with how many versions it has and where it stands), which open on their own page. Along the page's top edge is the mission over time: each crew's runs as bars in its colour, the one running now the plainest, and what mattered (a step accepted, a review, a decision, something asked) as ticks over them; a tick says what happened then on hover, and moments too close to tell apart share one. Under it, one bar stays as the page scrolls: back to the missions, what the mission's doing (the icon the missions list gives it), its name, and the tabs. *Summary*, *Log* and *Talk* are read in a column; *Brief*, *Work* and *Verify* take the page's width. *Summary* leads with where the mission stands as one block: the step it's on, the headline and what moves it on, and what waiting crew last said. The details (the state, the plan's steps or the brief's stages, outputs, capsules (with their git state and PR) and crew at work) are beside *Summary*; with them showing, *Summary* keeps to that block, the goal and what happened lately, and `j`/`k` there move through them. With them hidden, *Summary* has the plan, a *Work* row (the capsules in hand, and a line for what's up next, what's done and how many outputs, which goes to *Work*) and the crew at work now; `j`/`k` move through the capsules in hand. `s` hides or shows them there (remembered in your browser). On another tab `s` slides them in over the page, so the tab keeps its room, and `s` or `esc` slides them away. On a narrow window they go under *Summary*. A step in the details is a row: how it stands, its number and its name in a few words (its bold lead or first sentence), and at the row's end its kind (work, verify, review, decide, a pull request, land, wait), in the accent when it's yours. Pointing at one shows its card beside the list: the whole step, who's on it, what it makes, its checks and how each came out and, once it's passed, why, how long it took and the bundle that showed it. The step the plan is on is opened in place where that says more than its row does: what it makes, its crew and what they're doing, its clock, each check. A plan of more than nine steps shows that step with the two before it and the three after; the rest fold into a row on either side, a pip a step, which says what's worth knowing of them (how many were passed with a check unmet; how many to come are yours, and which is next) and opens them when clicked.

**The Brief tab** is the mission's brief and its plan as documents. The brief is edited as it reads: click into it and type, and *Save* (`mod+⏎`) writes what you changed. The plan is read, not typed in: ws holds it to a strict form, so what's offered of it is *Rewrite from the brief*, which has its planner write it again to fit the brief as it reads now (your edits to a written brief are saved first; briefing, what you changed in its draft goes to the planner with the ask, and it proposes the two together again). Under way that's `ws mission replan`: what it changes applies, or comes back to you to accept, as any revision does. *Edit by hand* opens the plan to type in all the same, until it's saved or your edits are discarded; one ws can't read is refused, with every problem shown under it. It has the page under the mission's bar to itself, in three panes that each scroll by themselves. Down the left is what the documents are made of: their sections, and the plan's steps as the details list them (a row each, with its card on hover), which follow the plan as you type it; each goes to its place. In the middle are the documents: the brief over the plan, or the two side by side when the window has room for both. Down the right is its planner's conversation, the one it was briefed in and carried on through it: what you and it have said, its steps folded over each reply, and a field at its foot to ask it (`/`; pick a passage out of the documents first, and `a` or *Ask about this* asks about that). `p` folds the conversation away, to a field at the documents' foot, and brings it back (remembered in your browser). A mission briefed without a conversation has a field there too: say what should change in the plan, and ws puts it to a planner (as `ws mission replan` does). One bar over the panes says where the documents stand, holds one toolbar for whichever of them you're editing, and has what there is to confirm.

What the planner proposes for the brief or the plan isn't a card in the conversation: it's a line there, and the documents show it, its changes since the draft before (briefing) or to what's written (under way) marked in them. The outline counts the changes still to read, in all and under each section; `j` and `k` step through them, `a` accepts the one in hand and `d` declines it (each has its *Accept* and *Decline* in place too), and *Accept all* takes the rest. A document whose changes are all settled is yours to edit. *Confirm and start* (briefing), *Accept the plan* or *Accept the edit* (under way) settles the proposal as you left it, your edits and the changes you declined with it; *Decline* declines it. Anything else its planner proposes (crew to start, a decision, a mission) is its card in the conversation, to confirm there. The tab's name carries a count while its planner has something for you: what it proposed, or your turn to answer.

Until there's a draft, the conversation has the documents' place: *Brief it* starts it, and the planner's questions and your answers are the page. *Write it yourself*, there before it's briefed and at the conversation's head after, opens the brief to write as your own, with the planner beside it: *Write it from the brief*, on the plan, then has it write the plan from yours. Under way, the plan says where the mission is on it: the step it's at is marked, with who's on it, and the steps behind it are marked done (a revision changes only what's ahead). An ended mission's brief and plan are read, not edited. On a narrow window the tab is one column down the page: the documents, then the conversation, with the bar staying at the top. `/missions/<slug>/brief`, where the brief had a page of its own, goes to the tab.

*Work* is a list, and the row that's open beside it. Capsules come first, in the order they need you: *In hand* (something's happening in it, or it holds work that hasn't landed), *Up next* (lifted, with nothing done in it yet) and *Done* (landed, merged or burned), which folds to a line. A capsule's row is its repo (and its own name, when that isn't the mission's), how much has changed, who's in it now and where it stands: crew asking or at work, a step that's yours, a pull request to open or as it stands, landed. Under the capsules are the mission's outputs, documents first, then handoffs, newest first. The tab opens on the capsule the mission's step is in, until you pick a row. A step is in the capsule its crew worked in, or the one its plan names (`in:`); a Review, Verify, Land or Open Pull Request step with neither is in the capsule of the step before. An open capsule has what you can do with it (its files, its review, opening its pull request, handing it on or dropping it), the step the mission's on when it's there, the crew who've been in it with the handoff each left, the plan's steps that ran there with their bundles (how one passed is said on hover), its commits and, beside those, the files that differ from where it left the default branch, by folder. Files not committed yet are among them, marked; lockfiles and generated files are quieter; a click opens the file in the capsule, with what's changed in it marked. An open output is rendered to read, and `⏎` opens its own page. On a narrower window the files go under the rest, and then the pane under the list.

**Artifacts** that are files here (proposals, notes, handoffs, reviews, files in a capsule or on the default branch) open in the web UI: markdown rendered to read and comment on (see *Threads on a document*), anything else as highlighted code. What they say is shown, never run: no HTML, no scripts, no script links, and Mermaid in its strict mode. Markdown is read as it's written for GitHub:

- **Tables, task lists, highlighted code,** and ```` ```mermaid ```` diagrams drawn.
- **Links between files.** A link to another file in the repo opens it in Files, in the same worktree (the capsule, or the ground), at its heading if it names one (`../adr/0009-state.md#decision`). A link to a heading in the same document goes there. One that leads out of the repo is marked, and goes nowhere.
- **Images** that are files in the repo are read from there.
- **Alerts.** A quote that starts `> [!NOTE]` (or `TIP`, `IMPORTANT`, `WARNING`, `CAUTION`) is drawn as one, in its kind's colour.
- **Maths.** `$E_{gh}$` in a line and `$$…$$` on lines of their own are drawn with KaTeX, loaded when a document first needs it. Single dollars follow GitHub's rule, so `$5 to $10` and `$HOME` stay as written.
- **Front matter** (YAML between `---` at the top) is folded away above the document. In the editor it's kept as written, as maths is: an edit beside either doesn't touch them.

**Conversations.** `ws talk` is for thinking something through with an agent before, or beside, the work: a problem, an idea, a question about the code. The agent reads anything here (the repos, the missions, GitHub) and changes nothing of the workspace's: what's to be kept (a decision, a mission, a document, an artifact), it proposes, and you confirm.

On Claude Code it runs things too, so "how slow are our tests?" is answered by running them. Its shell is in Claude Code's sandbox: it runs what it likes there (the tests, a build, a script that measures), reads everything and reaches the network, and writes in a folder of its own, the system's temp folder, and the capsules it lifts, nowhere else in the workspace. A repo's ground and a mission's capsules stay as they are. To change a repo's files it lifts a capsule (`ws lift <repo> <name>`, from the default branch or one it names): the conversation's own, not a mission's (not even on a mission's Talk tab: a mission's capsules are its plan's), and out of your editor. `ws show w3` lists a conversation's. Put the conversation away and its capsules are put away with it: burned, their branches kept, with what wasn't committed committed there first; taken up again, its agent lifts one back as it was. What it tries there goes on as a mission: it commits, and proposes one whose brief names the branch. `ws` and `gh` run outside the sandbox, only the ones a conversation may run, each on its own; one that isn't is refused with what to do in its place. It's loose on purpose for now (every host, every socket), to be tightened from what goes wrong. On Linux the sandbox needs bubblewrap and socat; without them, and on the other agent CLIs, a conversation's agent only reads.

```bash
ws talk "why is billing slow?"   # a new conversation about the workspace, opened in the web UI
ws talk                          # the list of them
ws talk -m token-cache "should stage 2 be split?"   # about a mission, on its Talk tab
ws talk --print "..."            # print the link instead of opening it
```

- **Refs.** The workspace's conversations are `w1`, `w2`, …; a mission's are `t1`, `t2`, …, numbered with its documents' threads. `ws show w3` (or `ws show t2 -m <mission>`) shows one, with its summary and latest turns, so any agent can pick it up.
- **What comes out of one.** The agent can propose a decision, a mission (as an idea, or with its brief: confirmed, it's created to brief, holding that brief, and its planner plans it in the mission's own conversation, where you confirm the plan, which starts it), a document of one of the workspace's [kinds](#wstoml) (*Add doc*, *Add plan*: added as `ws doc add` adds one, below) or an edit to one that's there (*Save the edit*, the card showing what it changes: saved as `ws doc amend` saves one, below), an artifact or a new version of one (*Add artifact*, *Add as v2*: running in a small frame under its reply, see [Artifacts](#artifacts)), and, in a mission's conversation, an edit to the brief, crew to dispatch, or a replan (`ws mission replan --propose "<what to change>"`: confirming asks the mission's planner as `ws mission replan` does, as you, and *Open it* goes to its conversation). A message that asks for several things has each placed: the agent names each ask and where it goes (a replan by default when it's this mission's work, an edit to the brief or a decision, or a new mission when it's clearly separate), and proposes it there. A proposal is a button under its reply: nothing happens until you confirm it, there or with `ws conversation accept <conversation> <proposal>` (`decline` to say no). *Edit* on one changes it first: a document's text, in the editor. A proposal drafted again (the same brief or plan, a mission or a document of the same title, an edit to the same document) supersedes the open one before it, which says so and is nothing to confirm: only the latest is. Once everything the agent's latest reply proposed is settled, the conversation isn't your turn any more (a declined one has the agent answer, which is). In a mission's conversation, what's open to confirm puts the mission under **Needs you** (`↩ to confirm`: *t3 proposes crew and a decision*); a conversation you've resolved asks nothing of you, though you can still settle what's in it.
- **Keeping the list short.** `ws conversation list` lists the open ones (`--all` for resolved ones too, `-m` for a mission's). `ws conversation resolve w3` puts one away, and `reopen` takes it up again.
- Conversations run on [`[conversation]`](#wstoml)'s harness and model, as threads and cells do.

**Documents anywhere.** Any markdown file in one of the workspace's repos is a document, whether or not a mission recorded it, and it's edited as a page: *Edit as a page* in Files (`⏎` on the file that's open). See *A file as a page* under Pages.

- **Read,** it's at its one address, `/documents/<repo>/<path>` (`?in=<capsule>` for its version in a capsule), where links between documents, Find and the old `/docs?ref=` lead: rendered, with its kind, the page it's edited in, and *Edit* (`e`), which edits it there (`?edit`), as the page tied to it, made the first time; *Read it* goes back. A page that isn't tied to a file is at `/documents/local/<id>`; `/pages/<id>` leads to whichever is its.
- **A save** goes where the file is:

  | The document is | Save |
  |---|---|
  | in a capsule | Commits the file there, as you (unless it has uncommitted changes of its own, which it's written beside). |
  | on the default branch of a repo with [`landing = "direct"`](#landing) | Lifts a capsule for it, commits the file as you, and lands it: one commit, straight to the default branch. The commit's message is shown to change before it goes, since it's the only description there'll be. If the repo's `verify` fails, or the push is refused, nothing is saved. |
  | on the default branch of any other repo | Lifts a capsule named for the document (`edit-<name>`), commits the file as you, and leaves it there: it's yours, to add to or open a pull request from. The page goes on with the file in that capsule. |

  The capsule is lifted from the default branch as it is on the remote, so a save made from an older ground is merged with what's landed since.
- **A file a mission records is that mission's document:** opening it from Files goes to the mission's page for it, where its threads are, and it's edited there.
- **Threads on files outside a mission** (`w12`) aren't made any more: a question or a change is asked on the file's page (*Ask* and *Change* on words you select, `>?`, passes). Those made before are kept to read, in `.ws/conversations`, and `ws show <repo>:<path>` lists them; they take no more replies or suggestions, and `ws thread` commands refuse them.

**Kinds of document.** A workspace can say what kinds of document it has ([`[documents.<name>]`](#wstoml)): where each goes and what it's called there, the first line of the commit that adds or changes one, and the file that says how one's written. A document is of a kind when its path fits the kind's, in a mission or not.

- **Its page says so:** the kind's name where *Document* would be (its title says where they go, and its guide).
- **A save starts from the kind's message,** as the commit's first line, in place of *Edit &lt;path&gt;*: in a capsule, and on a default branch. So does accepting a suggested edit, with the thread named in the commit's body.
- **Agents are told.** Every agent `ws` starts hears which kinds there are, and that `ws show kinds` says where each goes, its messages and its guide. One answering a thread on a document of a kind is told what kind, and to read the kind's guide before it answers.
- **`ws show <repo>:<path>`** says the kind, and `ws doctor` says when a kind's guide isn't there.
- **`ws doc add <kind> <file>`** adds one from a markdown file (`-` for stdin): named from its first heading, where its kind says, committed with its kind's `add` message, and saved as any document on a default branch is (landed, in a repo with no pull requests; else committed in a capsule lifted for it, `add-<slug>`, and left to you). `--dry-run` says where it would go and what it would be committed as. It's yours: crew write theirs in their capsule, and a conversation's agent proposes one (`--propose`) for you to add, the commit's body naming the conversation, which then lists it as what came out of it. The agent is shown the kind's guide before it can propose one: `ws doc add --propose <kind>`, with no file, prints the guide and its version, and a proposal of a kind with a guide is refused unless it gives that version (`--guide`), the guide printed with the refusal. The card says which guide it was written to.
- **`ws doc amend <ref> <file>`** changes one where it is (`records:docs/todos.md`; `repo/capsule:path` in a capsule) to the whole of a markdown file, saved as an edit on its page is: committed with its kind's `amend` message, in its capsule, or on a default branch landed (no pull requests) or committed in a capsule lifted for it (`edit-<name>`) and left to you. It isn't moved, whatever its first heading says. It's yours, as `ws doc add` is: a conversation's agent proposes the edit (`--propose`). It's shown the document first: `ws doc amend --propose <ref>`, with no file, prints it as it is, its kind's guide, and its version, which the edit's proposed with (`--on`); one without it, or with an old one, is refused, the document printed again. The card shows what the edit changes. If the document's changed by the time you accept it, the edit's merged into it from the version it was made on, as a page's save is; where both change the same lines, nothing's saved, and you ask for it again. A draft of it you have open on its page stays.

**Review.** A capsule is reviewed in the web UI: from its mission's page (`v` on a capsule there), or the headline when one's waiting for you. A review shows the capsule's committed changes since it left `origin/<default branch>`, file by file, with syntax highlighting, keyboard first (`?` lists the keys):

- `j`/`k` move line by line, `n`/`p` change by change, `J`/`K` file by file, and `t` jumps to a file by a few letters of its path.
- `x` marks a file viewed, folds it away and moves on. Viewed files are remembered in this browser, by content: a file that changes again comes back.
- `⏎` on a gap shows the unchanged lines it hides (and `e`, 20 more lines around the change you're on); a large file (over 3,000 changed lines, a lockfile say) waits for `⏎` too.
- `u` switches between unified and side by side.
- **The files are listed down the side by folder,** each folder named once over its files, so a row is a file's name. A folder too long for the list is cut from its left, and the list's edge drags wider. They're in the order they're read in: a folder's files together, a file then its tests (`cache.go`, `cache_test.go`), and last, under *To skim*, lockfiles, generated files (those the repo's `.gitattributes` marks `linguist-generated`, and names such as `*.pb.go` and `*.snap`) and files that were only renamed. Those are folded, and say why; `⏎` opens one, and they still count until you mark them viewed.
- Work the crew hasn't committed isn't in the review, and the review says which files have some. When the crew commits while you're looking, it says so, and `r` reloads.

- **The overview, the guide and passes.** A capsule's review has what a pull request's has (below): views beside the changes, findings in the diff to take (`⏎`) or dismiss (`D`), and the files in the guide's order if you choose. Its overview is its mission: a link to it, where it stands, and its brief; it has no *Discussion*. What the work is meant to do is the mission's brief, so that's what the guide compares the change with, and what a pass is told the work is for. Nothing's docked: the capsule is there. A finding you take goes into your draft, and to crew when you submit; one on no lines is under *The change as a whole* in what they read. Follow-up isn't one of a capsule's passes: crew answer a capsule's comments themselves.
- **Comments.** `c` comments on the line you're on, or on the lines you've selected. Select lines by dragging down the line numbers (dragging over the code still selects text, to copy), clicking a line number then shift-clicking another, or with `⇧↓`/`⇧↑` (or `V`, then move); a bar under the selection says which lines and offers *Comment*. Side by side, the column you drag picks the side: the left one comments on the removed lines. A selection stays within a file and doesn't cross hidden lines. Each comment is a kind: **blocking** (must change), **suggestion** (a judgement call), **question** (wants an answer, not a change) or **nit** (only if it's cheap); `tab` switches between them as you write. *Suggest a change* adds a ```` ```suggestion ```` block with the lines as they are, to edit into what they should be. `⌘⏎` saves. Comments are a draft until you submit: `⏎` on one edits it, `D` deletes it. Viewed files are kept with the mission, so they're the same in any browser.
- **Submitting.** `s` asks for a verdict, `a` approve, `c` request changes or `m` comment, and an optional summary, then `⌘⏎`. Requesting changes needs at least a comment or a summary; approving needs nothing. The review is then the mission's: it's kept as `mission:reviews/r1.md` (an artifact, so crew and the orchestrator find it), logged, and the orchestrator runs with it. It decides what follows (resuming the crew that did the work to address it, dispatching fresh crew with the review to read, or on an approval accepting the stage), but never changes the review: crew address comments, and you resolve them.
- Crew read reviews with `ws review show` (the latest of the capsule they're in, as markdown: each comment with the lines it's on, as they were) and `ws review list`, and are told to commit before reporting done, since a review is of commits.
- **Answers.** Crew answer each comment by its id once they've committed: `ws review addressed r1.c1 "what they did"` (recording the commit), `ws review wontfix r1.c2 "why"`, or `ws review reply r1.c3 "…"` for a question. Each comment shows its state (open, addressed, won't fix, resolved) and its thread, live. Only you resolve: `x` on a comment resolves or reopens it, and `⏎` replies.
- **Changes requested means you look again.** While a capsule's latest review asks for changes, `ws mission accept` refuses its stage, for the orchestrator and you alike: crew addressing a review isn't you being satisfied with it. Once crew have committed and reported done, the capsule is back with you: its mission's headline says it's ready for your re-review (`↻ re-review`, which goes straight to it). The orchestrator notes that it's waiting for you, rather than accepting or flagging.
- **Re-reviewing** opens on what's changed since your last review (`i` switches to all the changes and back). The last review's comments sit on the left, on the lines they were made on, so you see what became of each; comments on files that haven't changed since are listed after the files. Approve, and the stage can be accepted.

**Reviews** (*Reviews* in the sidebar, `g r`) lists the open pull requests across the workspace's repos, and reviews them with the review a capsule gets. Your teammates' come first, and yours sit below them.

- **Theirs are grouped by what each asks of you:** *Needs your review* (your review is asked for, by name first, then of a team you're in, and you haven't reviewed its latest commits), *Back with you* (you asked for changes or commented, and they've pushed or replied since), *Waiting on them*, *Approved*, *Everyone else's*, and, folded, *Drafts* and *Automated* (a bot's, unless it asks for your review).
- **Yours are grouped by where each stands:** *Returned to you* (changes requested, or its checks are failing), *Ready to merge* and *Waiting for review*. One from a mission's capsule opens its mission; one with no mission opens on GitHub, where it's merged.
- **A row** says who, which repo, how big it is, its checks and how long since it last changed. `⏎` opens the review, `o` opens it on GitHub, `/` filters by text, repo or author, and `r` asks GitHub again.
- **A stack is one row.** A stack of pull requests (GitHub's, or a chain ws works out from each PR being based on the last one's branch) is named for its bottom layer and sits where its most pressing layer would. Its layers are listed under it, bottom first, each with its own state; `l` folds them. `⏎` opens the review at the lowest layer that needs you.
- **The sidebar's count** is what needs your review or is back with you, and your own PRs returned or ready where no mission already says so. It's as GitHub last said: GitHub is asked while a Reviews page is open (every couple of minutes), and otherwise only when who's asking you by name has changed (below).
- **One that asks you by name needs you.** A pull request that asks for your review by name, and isn't a draft, is in the sidebar's **Needs you**, under *To review*, after your missions and conversations: it can't move without you. One a team's assignment gave to you asks you by name, so it's there; one asked of a team you're in isn't, as someone else can answer it. A stack is one row, opening at its lowest layer that asks you; past four, the rest are a press away in Reviews. The daemon keeps this current with no Reviews page open: a minute after it starts and every five after, it asks GitHub who's asking you by name (one search for each owner your repos are under), and asks for the whole list only when that's changed. A review you post takes its pull request off at once.
- **Not mine.** On the review of one that asks you by name, *Not mine* takes your name off its reviewers on GitHub, after asking: it's said there, where its author sees it, and nobody's asked in your place. It stops needing you at once.

**Reviewing a pull request** (`/reviews/<repo>/<number>`) is the page a capsule's review is: the same keys, viewed files, comments by kind and a draft until you submit.

- **The diff comes from git.** ws fetches the PR's head into the repo's clone, out of the way of branches (`refs/ws/pr/<number>/head`), and compares it with where it left its base. No capsule is needed, and the lines around a change open as in a capsule's review.
- **A stack's layers** are down the side, bottom first. Each shows only its own changes, from the layer below; `[` and `]` move between them. *Whole stack* (`w`) shows every layer together, for reading: comments are made on a layer. A layer whose layer below has moved on without it says it needs a rebase, and still shows its own changes.
- **Views.** At the top of the list down the side are *Overview*, *Guide*, *Discussion* and *Passes*, then *Changes*, with the files under it. One shows at a time, and `1` to `5` switch between them; each keeps its state while another shows (where it was scrolled, the diff's cursor, a comment half written), and the one showing is kept in the address, through a reload and moving between a stack's layers. The diff's keys work while *Changes* shows; picking a file in the list, or following a finding from *Passes*, shows it. The page opens on the overview. On a narrow window the views are tabs under the header.
  - **Overview** is the description as its author wrote it (a template's HTML comments left out). A whole stack's is each layer's description, bottom first; a whole stack has only *Overview* and *Changes*.
  - **Discussion** is what's been said on the pull request itself, on GitHub: who approved, asked for changes or commented and what they wrote with it, and the comments that aren't on lines, in the order they were made, bots' folded. A comment written there posts to GitHub at once, as you. What others have said since you last looked is a dot beside it in the list.
- **Where each other reviewer stands** (approved, asked for changes) is beside the pull request's status, dimmed when it's of earlier commits.
- **Other people's comments on lines** are GitHub's review threads, on the lines they're on. `⏎` replies and `x` resolves or reopens, each posted at once.
- **The guide** is a view of its own: *Write the guide* has an agent say what the change does (from the change, not its description), **where that differs from what it says it does** (what the description claims that the change doesn't do, and what the change does that the description doesn't say), and the order to read its files in, in groups, each with why it's read there and a line on what the change does to each file. It's written from the description and the change alone, so nothing's checked out for it. While it's written, the view shows it as it comes (what the change does, then where it differs, then each group as its files are placed), with how long it's been and what it was given: how many files, and those it wasn't shown (lockfiles, generated code, binary or too large ones). Claude Code says it as it writes it; with cursor-agent or pi (not over ACP), the guide shows once it's done.
  - **Each file's line is on the file.** What the guide says the change does to a file is under its name in the diff, whole, until you mark it viewed, in either order.
  - **The files can follow its order.** *Read the files in this order* (or *Guide*, at the top of the list of files) puts the list and the diff in the guide's groups in place of folders, each group named in the diff above its first file with why it's read there; the cursor stays on the file it's on. Viewed files are kept by content, so switching loses nothing.
  - **Nothing's hidden.** A group the guide says to skim (a rename carried through, generated code) is folded, as a lockfile is, and still counts until you mark it viewed. A file it doesn't place is under *Not placed*, last. When the pull request has been pushed to since the guide was written it says so, and the files it doesn't place come first, under *Not in the guide*: they may be what's new.
  - **How your workspace wants to be walked through a change** is `[reviews.guide]`, in `ws.toml` or `ws.local.toml`: `looks` in a few words ("migrations first, then what reads them"), or `file`, a document that says, and `auto = true` to have it written for every pull request that needs you.
- **Passes** are a view of its own. A pass is an agent reading the whole change and saying what it finds: a line or two on how the change stands, and findings, each of a kind, on the lines it's about, with the code it would put there where it has some. A finding may be on no lines: it's about the change as a whole ("this should be two pull requests"). *Run* runs one; it changes nothing, never runs the change's code and never posts.
  - **Go or no go is worked out, not stated.** A pass says which of its kinds block. It's *No go* while a finding of one is open, *Go, with notes* while only others are, and *Go* when nothing is, and it moves as you settle them: dismiss the last blocker and it's go, saying what you overruled. How each pass stands is a mark beside *Passes* in the list, from any view. Approving while one is no go is said in the submit dialog, and allowed.
  - **A finding shows in the diff,** under its lines, as a dashed card with its pass's name: nobody's said it yet. `⏎` takes it: it goes into your draft as a comment marked as the agent's, to edit or delete like any other, and is posted only when you submit. One on no lines goes in your review's body. `D` dismisses it, then `1`, `2` or `3` for why (wrong, said already, not worth it), which its agent is told when it's run again. `a` asks an agent about it. The same are buttons on the finding in *Passes*, where each pass's findings are listed, a link to its lines on each; *In the diff* hides a pass's findings from the diff.
  - **Run again,** a finding is the same finding when it's the same kind on the same lines as they read, wherever they've moved to, so a rebase doesn't reopen what you settled. A pass says when the pull request has been pushed to since it ran.
  - **Follow-up** is every review's, once you've reviewed. For each comment of yours that's still open, it says whether what's been pushed since deals with it (*Dealt with*, *Partly*, *Not dealt with*), and where. That's said on the comment in the diff, beside *Resolve*, and listed in *Passes* with *Resolve the comment*. Resolving the comment settles it. It's no go while a blocking comment of yours isn't dealt with. Nothing's docked for it: its agent reads the repo's clone, where the commits before and after are.
  - **A pass is yours to define,** in `ws.toml` (the team's) or `ws.local.toml` (yours, added to the team's):

    ```toml
    [reviews.passes.security]
    title  = "Security"
    about  = "What an attacker could do with this change."
    skill  = "security-review"            # or file = "…", or looks = "…"
    args   = "the changes in this pull request"
    kinds  = ["vulnerability", "hardening"]
    blocks = ["vulnerability"]            # open findings of these make it no go
    cold   = false                        # true: the change alone, nothing checked out
    auto   = false                        # true: run it over a pull request that needs you
    repos  = ["api"]                      # only these repos' reviews; all, left out
    ```

    `looks` says what to look for in a few words; `file` is a checklist its agent reads first; `skill` is one of the workspace's skills, followed for `args`, reporting in ws's form in place of its own. With no `kinds`, findings are a comment's kinds (blocking, suggestion, question, nit), and with no `blocks`, `blocking` blocks.
  - **Running by themselves.** A pass with `auto = true` (and the guide, with `auto = true` under `[reviews.guide]`) runs without being asked over the pull requests in *Needs your review* and *Back with you*, while a Reviews page is open: one at a time, and again once a pull request has been pushed to. One that fails isn't tried again until you run it. It's never run over a bot's pull request or a draft, and over a fork's only if it's cold. The Reviews list then says how each pass stands on the pull request's row (*Check: no go · 1 blocking*), dimmed when it's of the commit before, so you know before you open it. A pass that reads the code docks each pull request it runs over, which costs more than a cold one: `auto` is per pass.
  - **A pass that reads the code has the pull request docked first,** as asking does (a stack at its top layer), and starts from `ws show <repo>#<number>`, which says what's been said in review and what other passes found, so it isn't said again. A **cold** pass is given the description, what's been said and the change with its lines numbered, and nothing else: nothing's docked, so it's quick, and it works on a fork. Lockfiles and generated files are left out of what it's given, and it says so. One that reads the code is pointed at a diff that leaves out what the repo's `.gitattributes` marks `linguist-generated`, and told which files those are.
- **Submitting posts to GitHub, as you.** It's the one thing here that leaves your machine, so `s` shows exactly what will be posted, and where, before `⌘⏎` posts it. A comment's kind is its first words, as [Conventional Comments](https://conventionalcomments.org) writes them (`issue (blocking):`, `suggestion:`, `question:`, `nitpick:`), and a suggested change is a ```` ```suggestion ```` block GitHub can apply. A comment on lines outside the PR's diff on GitHub goes in the review's body instead. A stack's dialog lists its layers, each with its own verdict (`⇧A` approves every layer); a layer with none is left alone. Your own PR takes comments, not a verdict.
- **Looking again.** When they push, the review opens on what's changed since your last one (`i` for all of it). ws keeps the commit you reviewed (`refs/ws/pr/<number>/reviewed/<id>`), so it's there after a force-push; a review you made on GitHub counts too. After a rebase, each file's change is compared with the one you reviewed (`git patch-id`): a file whose change is the same is marked unchanged and folded, so a rebase that changes nothing shows nothing to re-read.
- **Ask** from the bar at the foot of the review (`/` goes to it): about the whole PR (`⇧A`), the line or lines selected (`a`, or *Ask* beside *Comment*), or what a pass found (*Ask* on the finding). ws docks the PR first (a stack at its top layer), so the agent reads the code as files with the rest of the workspace beside it, starting from `ws show <repo>#<number>`. It reads, and that's all: it never runs the PR's code, and never posts. The answer is a card over the review, with the agent's steps as it reads; what you type while it's open is a follow-up, and `esc` drops it. It can propose a review comment (`ws pr comment --propose`), which shows in the card with *Add to my review*: accepting puts it in your draft, marked as the agent's, to edit or delete like any other. Like an ask about a document, it's gone an hour after its last turn unless you *Keep it as a conversation*: then it's in *Conversations*, on the PR's page, and its agent may propose anything a conversation's can.
- **Capsules ws docks for a question** aren't boarded, don't run the repo's `after_create` hook, and are removed once the PR merges or closes, unless you've changed something in one. *Open here* (`⇧O`) docks the PR as `ws dock` does, boarded, and then it's yours.
- **A PR from a fork** is listed and reviewed, but can't be docked (`ws dock` looks for its branch on origin), so it can't be asked about.
- What ws keeps of a PR is in the workspace's records (`.ws/ws.db`): your reviews of it, the files you've viewed, whether it's docked, and what it says it does, so `ws find` finds it by its title or description (and *Said* in Find, where it isn't already listed). So is the list as GitHub last gave it, which a page opens on.

**Team** (*Team* in the sidebar, `g t`; `ws team` opens it) says what the people working in the workspace's repos have been doing, by day, newest first. It's about the work, arranged by people so you know who to ask: there are no counts, and nothing is said of anyone who did nothing.

- **What it reads.** Each visible repo's default branch, fetched into its clone and read along its first parents (a merged branch's own commits aren't each counted), and each repo's pull requests that have changed since ws last asked GitHub: opened, made ready, reviewed, merged, closed. What review comments say isn't read.
- **A row is one piece of work on one day:** a pull request, or commits pushed straight to a default branch with no pull request. One person's commits to a repo in a day are one row where they touch the same files (a record added, then amended twice), called by the first of them; commits to different files are rows of their own.
- **One change made in several repos is one row.** Pull requests on a branch of the same name, opened by the same person in more than one repo within two weeks of each other, are one piece of work: a dependency sweep across seventeen repos, a feature's backend and frontend halves. Its row says how many repos it's in and how many have landed (*In review, 9 of 17 landed*), what each person did and in how many (*Ana opened it in 17 repos. Ben approved it in 12 repos.*), and opens to its pull requests, each a link. It has one gist, written from all of them: when they're first seen, and again once none is still open. It's called what its pull requests are called, or by its branch where they differ.
- **Every row has** what it's called (its title, without its `feat:`), its repo, where it stands now (*In progress* for a draft, *In review*, *Landed*, *Closed*), who's on it (whose it is first, then who reviewed it that day) and, under it, what the change does, what's worth knowing of it, and what happened that day in a sentence: *Ben approved it. Ana landed it.*
- **What a change does** is its gist: one sentence an agent writes from the change itself (its title and description, the files it touches and its diff), saying the thing as it now is. A pull request's is written when it's first seen open, and again when it lands; commits pushed straight to a default branch have one too.
- **Worth knowing** is what someone who didn't write a change would be caught out by, or do their own work differently for. Most changes have nothing. Each is one of *Contract* (something other code calls or reads has another shape), *To do* (something you must do: a migration, a setting, an upgrade), *How we work* (a pattern, rule or decision to follow from now on), *Removed* or *Behaviour*, in a sentence, with the changed file it rests on, which opens in Files. One that names no file of the change is dropped, so what's said can be checked. What's only new (a flag, a field, a document) isn't one: the gist says it's there. Nor is what might follow from a change, as against what it makes so. A change that only touches dependency files (manifests, with a lockfile among them) has a gist and nothing worth knowing.
- **It's said under its change, once:** on the change's latest row, with the gist above it. *Worth knowing* (`n`) leaves only the changes that have something, each once, and says how many there are. The filter finds them by kind too: *contract*, *to do*, *how we work*.
- **A change to how things are done says so** with a mark on its row, whether or not an agent has read it: *Guide* (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, a template), *Skill*, *CI*, *Lint*, *Workspace* (`ws.toml`, `ws.repo.toml`). It's found from the files of what's landed, and an agent that reads the change is told those files are how things are done here. Watch paths of your own, for one repo with its name first:

  ```toml
  [team.watch.patterns]
  title = "Pattern"
  paths = ["docs:technical/patterns/**"]
  ```

  One named as a built-in (`guides`, `skills`, `ci`, `lint`, `workspace`) replaces it, and with no paths turns it off. The filter finds them by their mark: *guide*, *ci*.
- **Catch me up** (`c`) has an agent write what the team's been working on in the period: a few topics, each the changes that are part of one piece of work (a feature across a backend and a frontend, a sweep, decisions about one subject), with a sentence or two on what it comes to and where it stands. Who's on a topic is taken from its changes, not from what the agent says. A topic's name leaves only its changes showing (`esc` for everything again). It's written only when you ask, from the changes that have been read, and kept for the period (today, the last 7 days, all); when the period's changes move on it says so, with *Write it again*.
- **Since you last looked.** A dot marks each row that's had something happen since you were last on the page, a line sits under the last of them (*You'd seen what's below 2 hours ago*), and the top says how many changes are new. Being on the page a few seconds counts as looking.
- **By person** (`p`): the same changes, each once, under whose it is (what's still open first, then what's landed) and, under *Reviewing*, under whoever else was on it. People come in the order they last did something. Nobody is listed for having done nothing, and nothing is counted.
- **Near you.** A change that's landed and touches a file one of your capsules has changed too says so (*Also changed in your `api/token-cache`: cache.go*): it's what you'll meet when you next rebase. It's worked out from each capsule's changes since it left its default branch, committed or not.
- **When it's written.** While the page is open, changes since the start of yesterday are read as they're seen, two at a time. Older ones aren't until you ask: `w` on a row, or *Read N more changes* (`⇧W`) for everything showing. A gist is written once for what it's of, and kept. One that couldn't be written says why on its row, with *Try again*.
- **The agent** is given the change in its prompt and no tools, replies in a form ws reads, and its session isn't kept. It runs on the harness conversations run on, on that harness's small model (`haiku` on Claude Code). `[team] harness` and `model` pick another, and `[team] narrative = false` turns gists off: the page is then the facts alone. A diff is given up to 48 KB, 6 KB a file, without lockfiles and other generated files.
- **A merge is one thing,** whichever of git and GitHub knows of it: a commit on the default branch that names a pull request (`… (#12)`, or *Merge pull request #12*) is that pull request landing. Where there's no GitHub to ask, the page says so, and shows what git knows.
- **People are paired** across their login on GitHub and their name and email in git: by a squash merge's commit (its author is the pull request's), by GitHub's no-reply addresses, and by the same name. Someone is called by the name on their GitHub profile, where they've set one, or their name in git; a whole name before a first name alone. Where that isn't enough (a reviewer with no name on their profile is their login), say who someone is:

  ```toml
  [team.people.ana]
  name = "Ana Diaz"
  logins = ["ana-acme"]
  emails = ["ana@acme.example"]
  ```

- **Bots' rows are folded** at the foot of each day (*2 automated*; `a` shows them): GitHub's bots, and release tools that commit under names of their own.
- **Keys.** `j`/`k` move, `⏎` opens a row to its events (each with who, when, the commit and the files it touched), `o` opens it (an open pull request's review here; anything else on GitHub), `w` has a row's change read, `n` leaves only what's worth knowing, `c` catches you up, `p` arranges by person, `1`/`2`/`3` pick today, the last 7 days or as far back as ws has looked, `/` filters by text, repo or person, and `r` looks again.
- **Looking.** While the page is open, ws fetches and asks GitHub every couple of minutes, and not otherwise. A look goes back 14 days (`[team] window = "3w"` for more), and that's what's kept: a pull request that hasn't moved since goes, with its events. Asked for a longer period, it goes further, and keeps that for the rest of the day.
- **`ws team --since <when>`** prints a period in the terminal: `today`, `yesterday`, a weekday, `week` (since Monday), `7d`, `2w` or a date, with the gists that have been written. `--read` has every change in the period read first, where it hasn't been: an agent run each, so a fortnight of a busy workspace is a couple of hundred. `-f json` prints it as the page reads it, and `-f llm` for an agent: each change once, with where it is, where it stands, who's on it, what it does and what's worth knowing.
- **It's yours.** What's read and written is kept in the workspace's records (`.ws/ws.db`: the events, the pull requests as GitHub last said them, the gists, the accounts, when it last looked, and when you did), on your machine. Nothing is posted or shared. Each person's ws writes its own gists.

**Pages** (listed in *Documents* in the sidebar, `g d`; `ws page` opens them, and `ws page "Token cache"` starts one) are documents you write, before or beside any mission: a problem being worked out, with an agent asked in where you want one.

- **A page saves itself** a moment after you stop typing. Its text is a markdown file, `.ws/pages/p1.md`, local to your machine as missions are, and what it is beside that (the kind of document it's written as, where it was filed, the file it's tied to, the version saved) is in the workspace's records (`.ws/ws.db`). `ws page list` lists them with their files; `ws show p1` shows one, with its cells and passes and what it says. A save is refused if the page has changed since the version it was edited from, the file edited by hand included: what's edited by hand is taken in when the page is next listed, saved or searched for. *Remove* takes one out of the list and keeps its file (`p1.md.removed`); it's no longer found, and its number isn't used again.
- **A cell is something you asked for.** Start a line with `>>` and say what you want there (`>> a table of the existing routes`), press `⏎`, and write on. An agent reads the page and what it needs of the workspace (the repos, the missions, GitHub; it changes nothing), and what it writes fills the cell. While it works, the cell says what it's doing.
- **How a cell reads.** What's written into a page is written as the page's: no "I" or "we", nothing about what was searched or run, what's so said with the file it's so in, no guesses marked in the text. What its agent couldn't find, didn't check or had to guess is said to you instead, as its notes in the cell's panel (the cell's head says *With a note from its agent*), and isn't in the page.
- **A question** is a line that starts with `>?`: `>? does demo parse any flags today`, `⏎`, and write on. It's answered in its panel (click its head, which says *Answered*), where you can ask more. Nothing is written into the page for it: in the page it's only its head, and in the file two comments. *Put it in the page* (`⌘⏎`) has what was answered, or what you say there, written in, and it's a cell from then on. So does asking for it in words: "draft this in the doc", "add that as a table", said as any follow-up.
- **Ask for a change** by clicking the cell's head, or `⌘⏎` with the cursor in it: its panel has what you asked for, each version it wrote and what the latest was written from, and a box to say what to change (*add a column for how many lines each has*). *Change* (`⏎`) has the agent write the cell again, in the same session; *Just ask* (`⌘⏎`) asks about it instead, and the cell stays as it is. *Use this* puts an earlier version back.
- **What a cell holds is yours to edit,** as any of the page. Ask for a change to one you've edited and its agent starts from your edits; how it read is kept under what you asked (*With your edits to it*), with *Use this* to put it back. *Keep as text* drops the cell and leaves what it holds; *Remove* takes both out.
- **Your own words changed.** `⌘⏎` outside a cell (or *Change* on the page's bar) asks what should change in the paragraph the cursor's in, or the blocks you've selected: *tighten this*, *make it a table*. They become a cell, as they are until its agent has written them again, and the panel keeps *Your words, as they were* to put back. *Keep as text* when it reads right.
- **Select words** (not in a cell) and a popover under them has *Ask* and *Change*. *Ask* asks about them: a question about those words, quoting them, put after the blocks they're in and answered beside the page, as `>?` is. *Change* is `⌘⏎` on them.
- **Cells fill top to bottom.** One asked for under a cell that's still being filled for the first time waits for it (*Waiting for the cell above*), so "which of the files above" has them to read.
- **What a page names is linked.** Write `demo/main.go`, or `demo/main.go:12-20` for lines of it, `demo/internal/` for a folder, a repo's name (`demo`), `demo#12` for a pull request, a mission's slug, or another page's or a conversation's id (`p3`, `w4`), and it's marked where it names something that's there. Hover says more (a file's lines, a pull request's title and checks, where a mission stands), and `⌘`-click goes there: Files at those lines, the review, the mission. A mission is followed by where it stands and a page by its title. A repo is a pill in its colour, showing its display name; on a page, put the cursor in it to edit its name. *Link names* makes what a page names links, so it links on GitHub too: a file's lines at the commit they were read at, a pull request and a repo at their GitHub address, a mission, page or conversation as `ws:<name>` (plain words on GitHub). Undo takes it back. A cell's names are linked the same way as it's written in. A link to one of the workspace's repos on GitHub, in any document ws shows, opens in ws (Files, at the heading it names, or the pull request's review) with the same card; the file isn't changed. A plain click only puts the cursor there, and nothing's added to the page's file. A file in a capsule is named `demo/token-cache:internal/cache.go` (and `:12-20` for lines), and a repo's alias names the repo. A file named as it is on disk (`repos/demo/token-cache/internal/cache.go`, or that path in full under the workspace's folder) is read as the same file.
- **The same names are linked wherever you read.** A conversation, a thread, a pass's findings, a cell's notes, a handoff: each name is drawn as on a page, with the same card, and a click goes there. So is a link an agent made to a file by its path on disk, keeping its words. Every agent is told to write these names (`prompt.Naming`). A turn keeps what its names were when it was said, so a capsule's file it named is still drawn once the capsule's landed and gone: its card says so, and shows the file on the default branch.
- **What it was written from.** The panel lists the files the cell's agent read, each a link to it in Files. When one changes, the cell says so (*main.go has changed since*) and the list of pages says how many are out of date; *Write it again* has its agent read what's there now. A cell is also out of date when a cell above it changes (*A cell above has changed since*), A cell never writes itself again. Only files it read count: not what it learned from `git log` or a search.
- **More than one block is laid out first.** *Add* on the page's bar asks for something anywhere in the page (*a section after Constraints on X, and one after Release on roll out*); *Draft it* asks for a first draft of the whole page; and a `>>` asked for several sections does the same where you asked. One agent plans the blocks, and each heading goes in at once with an empty block under it saying what it'll hold. Then it writes them one by one (*To be written*, then what it's reading, then the block), while you go on writing anywhere. Each block is a cell from then on: change it, ask about it, keep it as text. One you take out before its turn isn't written. A plan only adds: it doesn't move or change what's there. 
- **Passes** read the whole page and say things about it. They aren't part of the page: each is a tab beside *Page*, and nothing a pass says is written into the page's file.
  - **Check:** what's open, ambiguous or missing, for someone who'd build from it. **Formalise:** what the page requires, as statements that could be checked, and what's too vague to check, with words that aren't. **Cold read:** what an agent given the page and nothing else (it can't read the workspace) would build, and everything it had to assume.
  - **A tab** has *Run* (then *Run again*), the pass's steps while it runs, what it said of the page, and what it found: each finding a kind, the words of the page it's about, and what it says of them. Its count is the findings still open.
  - **A finding** is dealt with by *Done*, *Dismiss* (as *Wrong*, *Already said*, or *Not worth it*), *Use these words* (the words it proposes go on the page in place of those it quoted) or *Find out* (an agent answers it there, under the finding, where you can ask more; nothing's put in the page unless you press *Put it in the page*, which has it written in under the words the finding is about). Those settled fold away, and can be opened again.
  - **Run again** reads the page as it is now. What you marked done or dismissed stays so, what the page has since answered is gone, and what's new is added. When the page has changed since a pass ran, its tab says so.
  - **Show on the page,** per pass, marks its open findings on the words they're about; a mark opens the finding there. They're drawn over the page, not put in it.
  - **Your own passes** go in `ws.toml`:

    ```toml
    [pages.passes.security]
    title = "Security"
    about = "What could leak a secret or widen who can do what."
    looks = "Read it as someone looking for how this could go wrong: secrets in logs, a permission widened, input trusted."
    kinds = ["risk", "question"]
    # cold = true gives its agent the page and nothing else
    ```

    A pass can take how it's done from a file instead of (or as well as) `looks`: `file = "path"`, from the workspace's root, a full path, or `~/…`. Its agent reads the file first: with the [humanizer](https://github.com/blader/humanizer) skill installed, `file = ".claude/skills/humanizer/SKILL.md"` and `kinds = ["tell"]` make a pass that marks where the page reads as if a model wrote it, with plainer words to put there.

    **A pass can be one of the workspace's skills,** asked something: its agent follows the skill for that, on the page, and what the skill would report (a verdict, a list) comes back in a pass's form, the verdict as what it said of the page and each item as a finding on the words it's about, of the kind the skill files it under. The skill is found by its name, as `ws show skills` lists it; a pass whose skill isn't there says so on its tab, and can't be run.

    ```toml
    [pages.passes.share]
    title = "Ready to share?"
    skill = "build-proposals"
    args = "ready to share?"
    ```

    **A kind's passes.** `passes = ["missing", "share"]` under a [kind of document](#wstoml) makes those of your passes tabs on pages started as that kind, and on no other page. The built-in passes are on every page.
  - **How well a pass does.** `ws show passes` counts, for each pass over every page it's run on (and each of a review's passes over every pull request), what came of its latest findings: taken, dismissed (wrong, already said, not worth it), or still open, and, of those judged, how many were right and how many acted on. A pass run again is told which of its findings were dismissed, and why.
- **How cells write** can be set for the workspace: `[pages] style = "path"` names a file (a style guide, a skill's instructions) that each cell's agent reads before it writes.
- **Kinds of document.** A workspace with [kinds of document](#wstoml) can start a page as one: *New as…* (`N`) in Documents, a kind and a title. Its agents are told what it's to be and the kind's guide, and *Draft it* lays it out as the guide says. Its kind shows on the page's bar and in the list.
- **Filing a page.** *File as…* on the page's bar files it as a document of one of the workspace's kinds: pick the kind, give it a title (the page's, unless you change it), and it shows where it goes and the commit's message, which you can change. What's filed is the page as it reads: what its cells hold is kept as text, its questions are left out, and nothing of ws's is in the file. It's saved as any document on a default branch is: landed, in a repo with no pull requests, or committed in a capsule lifted for it (`add-<slug>`) and left to you. A numbered kind's number is worked out from what's on the default branch then, and if someone lands that number first, filing takes the next and goes again. The page stays, with its cells and passes, tied to the file it was filed as: *Save to repo* saves it again (see *A file as a page*), and the document says what page it was written in. A cell still being written stops it.
- **A file as a page.** A markdown file in a repo is edited as a page: *Edit as a page* in Files. It's the one page tied to that file, made the first time with its text (and its [kind](#wstoml), if its path fits one), so the page's tools are all there: cells, questions, *Change*, *Draft it*, passes. The page's bar names the file.
  - **The page keeps itself** as you type, as any page does. ***Save to repo*** writes it to the file, as it reads (cells as text, questions left out), with the commit's message to change: committed in its capsule, or on a default branch landed (a repo with no pull requests) or committed in a capsule lifted for it, as a document is saved.
  - **When the file's changed** since the page took it in (someone landed a change), the page says so, and saving waits: *Bring in what changed* merges the file's changes into the page, with what you've written and its cells. Where both changed the same lines, both are kept, marked (`<<<<<<<`), for you to settle on the page.
  - **A filed page** is tied to the file it was filed as: *Save to repo* saves it again.
- **The map,** beside the page on a wide window (*Map* folds it): what needs you (open, out of date, not filled), the outline, each cell and where it stands, each pass that's run with its open findings, everything not ticked off, and the files the page rests on, each with how many cells rest on it. Each goes to what it names.
- **A cell that couldn't be filled** says why in its panel; *Try again* fills it.
- **In the file,** a cell is its markdown between two comments (`<!-- ws:cell w7 v2 | a table of the existing routes -->` and `<!-- /ws:cell -->`), so the file reads as plain markdown. A cell is one of the workspace's conversations: `ws show w7` has what was asked and each version.
- **What an agent writes is put into the page by the browser:** a cell that fills while its page is closed waits, and lands when you next open it. A page open in two tabs saves from one: the other says it's changed, and asks which to keep.
- Cells run on `[conversation]`'s harness and model: with no `[conversation] harness`, crew's harness (Cursor, by default) and its conversation model. `[conversation] harness = "claude"` in `ws.toml` or `ws.local.toml` runs them in Claude Code.

**Artifacts** are versioned HTML prototypes: something to look at and use rather than read, such as a mock-up of a page, two takes on a colour, or a diagram you can click through. An artifact is a page: one HTML file with its CSS and JS inline, or `index.html` with the files it loads beside it (stylesheets, scripts, images, fonts, data). It has versions, each whole, and each says who made it, where, what it's based on and what changed. It isn't a kind of document, and it isn't a mission's recorded artifact (`ws mission artifact add`), though a mission's list can point at one.

```bash
ws artifact add glow.html --title "Horizon glow" --note "Accent at 8% in dark and 3% in light."   # a3.v1
ws artifact add glow.html --to a3 --note "No glow in light: at 3% it looked grey."             # a3.v2, based on v1
ws artifact add glow.html --to a3 --on v1 --note "Softer, from v1."                             # a3.v3, based on v1
ws artifact add site/ --title "Site" --note "A page with its stylesheet and images."           # a folder: index.html and its files
ws artifact list                  # the newest version first (--all: discarded ones too)
ws show a3                        # its versions, what each changed, and where each came from
ws artifact show a3.v2            # v2's page (a3: the latest)
ws artifact show a3.v2 css/site.css   # another of its files (--files lists them)
ws artifact checkout a3 work/     # a version's files in a folder, to work on
ws artifact url a3.v2             # the address it runs at, to open in any browser
ws artifact add work/ --to a3 --note "Teal headings."   # the folder, as the next version
ws artifact tweaks                # how a page says what can be changed about it (ws.tweaks)
ws artifact promote a3.v2 --into glow   # keep it for good: v2 chosen, into mission glow, and put where it lives in a repo
ws artifact promote a3            # again, versions later: the latest goes to the same folder, in one commit
ws artifact discard a3 "v2 of a5 won"
ws artifact restore a3            # put it back
ws artifact clear                 # discard the expired now
ws conversation accept w9 2       # add what conversation w9 proposed (its proposal 2): a3.v2, from w9
```

- **Where they're kept.** On this machine, in `.ws/artifacts/{date}-{slug}/`: an artifact is a folder holding its record, `artifact.toml`, and a git repo of its own, `files.git`, where each version is a commit tagged `v1`, `v2`, …, whose files are the version's and whose parent is the version it's based on. So a file that doesn't change between versions is kept once. The repo is ws's: nothing's worked on in it, its commits are ws's own (unsigned, whatever your git signs with), and you read a version with `ws artifact show` or `ws artifact checkout`. The folder's named from the day it was made and its title (`2026-11-12-horizon-glow`; a second of the same name is `-2`), and keeps its name if the title changes. `artifact.toml` is the record: ws reads it into the workspace's records for listing, finding and refs, and where the two differ, the file wins (edit it by hand and ws reads it again). Nothing of an artifact is in a repo until you promote it, and then only the version you chose is (below): the rest of its versions, and every artifact that isn't promoted, are this machine's.
- **Where a promoted one lives.** A promoted artifact can live in a repo too, for everyone with it: its *home*, a folder there (`records:artifacts/2026-11-12-horizon-glow/`) holding the version chosen, its page as `index.html` with its other files beside it at their paths, a record of it, `artifact.toml` (what it is, that it was promoted and into what, and each version up to that one with its note, who made it and where), and a `thing.toml`, which says the folder is an artifact to any ws that has the repo. Where it lives is chosen the first time it's promoted, and kept in its record here: where `[artifacts] path` in `ws.toml` says promoted ones live ([ws.toml](#wstoml); the folder's named as that says, its own name where that's free), a folder you name (`--home records:kept/glow/`), or this machine only (`--here`), which is kept too, until you give it a home. Putting it there is one commit on the repo's default branch, with `promote` (*Promote artifact horizon-glow: v2*) and, in its body, who promoted it, the version chosen and what it went into, and where that version came from (*Proposed in conversation w9, "Radial horizon gradient…" (proposal 2), and added by you.*, or a mission, or by hand) with its note; it's landed as `ws land` lands one (the repo's check runs first), and nothing's left of one that stops. The repo has to take commits straight to its default branch (`landing = "direct"`): in one with pull requests a promoted sketch would wait in review. **Promote it again, versions later, and it knows:** the version chosen then takes the other's place in the same folder, in one commit, with nothing asked. It isn't moved to another folder. `ws show a3` says where it lives, which version is there, and whether a newer one's here; anything written for other people names the folder, as `a3` is this machine's.
- **Everyone with the repo has it.** ws finds a promoted artifact in a repo by its `thing.toml`, as it finds a plugin's things there ([Plugins](#plugins)). One that isn't this machine's (someone else promoted it, or you did on another machine) is named by where it lives, and is listed after this machine's own (`ws artifact list`; *In the workspace's repos* on the Artifacts page, by repo and by the folders they're in under where `[artifacts] path` puts promoted ones), found by its title and what its record says each version changed (`ws find`, Find), shown (`ws show records:artifacts/2026-11-12-horizon-glow/`, which says where its files can be read), and run in the web UI, in the same sandboxed frame, from the repo's files: nothing's copied onto this machine. It's as it is on the repo's default branch, as last pulled, and it's another version when any of its files changes, not only its page. It has no versions here, and isn't changed here: to work on it, add it as an artifact of your own (`ws artifact add <repo's folder>/index.html`).
- **From before.** A version used to be one file in its artifact's folder (`v1.html`, `v2.html`, …). Any from then are put in a repo of their own, each version a commit, by the daemon as it starts, by `ws doctor`, or when one's next given a version; until then they're read as they are. And ws used to keep artifacts in the repo whole, every version a commit. Any it knows from then are brought onto this machine once, by the daemon as it starts and by `ws doctor`: its folder as the repo's default branch has it is copied into `.ws/artifacts`, its ref the same. One that was promoted lives where it was; one that wasn't is this machine's only, and its old folder is left in the repo for you to remove (`ws doctor` names them).
- **Refs.** Each artifact is `a<n>` (`a3`), and a version `a3.v2`. They're this machine's, as `p3` and `w4` are. `ws show a3` gives the title, where it stands, where it's kept, and each version with what it's based on, who added it and where (a mission and its crew, or by hand) and its note, and how to read one. Agents are told to name them so, and wherever the web UI draws what's written (a handoff, a note, a conversation), `a3` and `a3.v2` are refs: their card says the title, which version of how many, where it stands, and the version's note, and a click opens it.
- **Versions.** Each is whole, never changed or renumbered once written. A version is based on the one it revises: the latest, or another with `--on`, so versions can branch, and they're still numbered in order. Each has a note of a line or two: what changed and why, and the first's what the artifact tries out.
- **What a version can be.** A page, `index.html` (it must start as HTML: `<!doctype html>`, then its page), with a title (`--title`, or its `<title>`), and whatever files it loads beside it, each at a path of its own (`css/site.css`, `img/logo.png`): 10 MB in all at most, and 200 files. `ws artifact add` takes one HTML file, which is the page alone, or a folder, whose `index.html` is the page; hidden files (`.git`, `.DS_Store`) are left out, and a link is refused. The web UI runs it in a sandboxed frame that loads the version's own files and nothing else, and keeps nothing. So ws warns when it loads a script, font or image from elsewhere, loads a file that isn't among its own, fetches from elsewhere, or uses storage (`localStorage`): put what it needs among its files. Its scripts can read its own files (`fetch("data.json")`), and a link to another of its pages opens it in the frame.
- **Working on one with files.** Each version is whole, so a change to one file is a new version with the rest as they were. `ws artifact checkout a3` (or `a3.v2`) puts a version's files in a folder: one you name, or a new one in the temp folder, which it prints. Change what you're changing there, and `ws artifact add <folder> --to a3 --note "…"` adds the folder as the next version. Nothing links the folder to the artifact.
- **Looking at one outside the web UI.** `ws artifact url a3.v2` (or `a3`, for the latest) prints the address a version runs at: the page the web UI frames, as it runs there, under the same policy (its own files, no network, no storage, sandboxed). It opens with no sign-in, so a browser that isn't signed in to ws can look at it: an agent's (Playwright, Chrome), or one you send it to. Its key is made from the workspace's token and the version, so it opens that version's files and nothing else, and stops working if the token changes. `--set name=value` sets one of its tweaks, as the tray would (`--set density=compact`). The daemon serves it, so it works while the daemon's running.
- **How its files are served.** A frame with an origin of its own sends no cookie when it loads a file, so a version's files can't sit behind the sign-in. They're under an address with a key in it (`/framed/<key>/a3/v2/css/site.css`): `/frame/a3/v2`, signed in, sends the frame there. The key is made from the workspace's token and the frame's address, so it reads that version's files and nothing else, and the frame's policy names that address as the only place the page may load from.
- **Tweaks.** Where a page has something to choose between (a density, a colour, two takes on a layout), it draws no controls of its own: it says what can be changed, and the web UI draws a tray beside its frame, a control for each tweak and a button for each preset. The page declares them with `ws.tweaks({ tweaks, presets })`, which ws writes into every frame ahead of the page's own scripts (the frame fetches nothing, so the page couldn't load it): each tweak has a name, a label and a description, and is a choice (`options`, each a string or `{ value, label, description }`), a switch, a number (a slider with `min` and `max`, and a `unit`), a text or a colour (`type: "color"`), told by its options or by the value the first preset gives it. The first preset is what the page opens as, and gives every tweak a value; any other sets the tweaks it lists and leaves the rest (how it is today, what the agent suggests, each treatment worth a look), and is on while they're all as it gives them. Each value is written on the page's `<html>` for its styles (`data-density="compact"`, `data-pips` there or not for a switch, `--radius: 12px` and `--accent: #5b8def` for a number and a colour), `ws.tweaks.state` is the state, and the page's window hears `ws:tweaks` as it changes. `ws artifact tweaks` prints how, and agents are told. The state is in the address where it isn't the one the page opens as (`/artifacts/a3/v2?t.density=compact&t.pips=off`), so a state can be linked, and *Reload* and *Open alone* keep it; a tweak that isn't as the page opens it is marked, the tray's head says how many are, and *Copy* copies what each is set to, as text, to say which you'd keep. A version's tweaks are its own: another version opens as its page says, and a state in an address is met by name, what the page doesn't declare or take dropped. What's wrong with a declaration (a tweak the first preset gives no value, a preset naming a tweak that isn't there) is listed at the foot of the tray, and the rest is drawn. ws reads none of it on the server: a version's file is as it was written, and what its page declares is known only by running it.
- **Crew** are taught to make one when the work calls for something to look at, or options to choose between. Theirs says it came from their mission (its slug and title, and the crew), must have a note, and is recorded on the mission as one of its artifacts (`[artifact, live] a3 — Horizon glow — ws show a3`), with the crew and the stage. It's on this machine like any other, not in a capsule: there's nothing of it to commit. The orchestrator makes none, and a conversation's agent proposes (below).
- **Yours,** added from your own terminal, has you as its only provenance (*by you, by hand*), and needs no note.
- **From a conversation.** A conversation changes nothing of the workspace's, so its agent proposes one, and you add it, or not, with one click. `ws artifact add --propose`, with no file, prints how one's written (its page, alone or with its files beside it, nothing from the network, both colour schemes where it matters, no `localStorage`); then `ws artifact add --propose --title "…" --note "…" -`, with the HTML on stdin, proposes it, checked first as any version is (a page, within the limits, with a title and a note), what won't load in the frame said to the agent then. One with files is proposed as the folder they're in (`ws artifact add --propose … <folder>`), which an agent whose shell is sandboxed writes in its own folder; what it proposed is kept beside the artifacts (`.ws/artifacts/.proposed.git`) until you add it. A revision is the whole file again, proposed with `--to a3` (and `--on v1` for one other than the latest), after reading it with `ws show a3` and `ws artifact show a3.v2`; for one with files, the agent checks the version out into its own folder (`ws artifact checkout a3.v2`), changes it there, and proposes the folder. The card under the reply shows the title, the note, where it would go, and the HTML running in a small sandboxed frame (served as a version is, from `/frame/w9/proposals/2`), with *Open larger*; a revision is shown beside the version it revises, and if versions have been added since it was proposed, the card says so (*Proposed on v1; v2 has been added since*). *Add artifact* (or *Add as v2*) adds it, as you, *from w9*, with the proposal's number and the conversation's title; *Decline* leaves nothing, and the proposal stays in the conversation with its frame. *Edit* changes its title and note first. A revision added after others is still based on the version it was proposed on, and takes the next number. Added, the card shows the version it made (`a3.v1`), its frame, and *Open it*; `ws show w9` lists it under what came out of it. In a mission's conversation, it's recorded on the mission too. The agent may run `ws show`, `ws artifact show`, `ws artifact list` and `ws artifact add --propose` (that, with `--propose` first), on every harness; told to add one itself, it's refused.
- **They're sketches,** and most should go: each is *live* until it's promoted, discarded, or it expires. Promoting, discarding, restoring and clearing are yours alone: crew say in their handoff which they'd keep or drop, and why.
- **Promoting** keeps one for good: it expires no more, and says which version was chosen (`a3.v2`, or `--version 2`; its latest for neither) and what it went into, if anything: a mission (`--into glow`, which records it among the mission's artifacts, so whoever briefs it and its crew read it), a mission started from it (`--start "build the glow"`, waiting to be briefed, with it recorded on it), or a document (`--into records:plans/glow.md`), which cites it by its folder in a repo, so the link works for everyone: it has to live in one. Where it lives in a repo, the version chosen is put there (above). A promoted one still takes new versions; promoting it again chooses another, or says what it went into now. `ws show a3` and its page say what was chosen and what it went into.
- **Discarding** takes one off the list, with its versions, saying why if you like (`ws artifact list --all` lists it). Its folder moves to `.ws/artifacts/.discarded/`, and is deleted for good 30 days later. Where it lives in a repo, its folder there is taken away first, in one commit, with `discard` and the reason in its body, landed. `ws show a3` still says what it was, and it takes no new versions. Its number and its folder's name aren't used again.
- **Restoring** (`ws artifact restore a3`, or *Restore* on its page) puts it back as it was, live, or promoted if it had been, out of `.discarded`, while it's there. Where it lived in a repo, the version that was there is put back, in one commit (`restore`), landed. Its expiry counts from then.
- **Expiry.** A live one expires 30 days (`[artifacts] expire_after`) after its last version, or after it was restored if that's later: a new version starts the count again, and promoting ends it. The list warns in its last week (*expires in 4 days*), with *Promote* beside it, and `ws artifact list` says it too. The daemon discards an expired one itself, by *ws*, "expired" its reason, when it starts and each day it runs (and deletes what's been in `.discarded` for 30 days); until it does, the one expired is listed as *expired*, dimmed, and *Clear expired* on the list, or `ws artifact clear`, discards them now. Each can be restored. Only a live one expires, and a live one is in no repo, so ws takes nothing out of a shared branch on its own.
- **Found with everything else:** `ws find` finds an artifact by its title and its versions' notes.
- **In the web UI,** *Artifacts* in the sidebar (`g a`) lists them, the newest version first: each with its title, its latest version (`v3`) and its note, where its versions came from (a conversation or a mission, drawn as refs), who added the latest and when, and where it's kept (*This machine*; or, for one that lives in a repo, the repo and the version that's there, *records · v2*, with the latest here where that's newer). *All* is the live and the promoted; *Live*, *Promoted* and *Discarded* keep to those (a discarded one is shown only there), and a menu keeps to what came from one conversation or mission, or was added by hand. One in its last week says when it expires, with *Promote* beside it; one expired is dimmed, and *Clear expired* clears them all. `j`/`k` and `⏎` open one. Find (`⌘K`) has them as *Artifacts*, by title.
- **An artifact's page** (`/artifacts/a3`, or `/artifacts/a3/v2` for a version) runs the version in a frame that fills the page under its head: the title and where it stands, the version's line (`v2 of 3 · from w9 · by you · 2h ago`, and *based on v1* where it isn't the one before), and its note. The versions are a strip, `v1 v2 v3`, each with its note on hover; `[` and `]` step through them. *Fit*, *1280*, *768* and *390* (`w`) draw it at a desktop's, a tablet's or a phone's width; *Reload* (`r`) runs it again, *Stop* (`x`) unloads it (a script that loops stops with it), *Source* (`s`) shows its HTML, highlighted, *Compare* (`c`) puts it beside another (below), and *Open alone* (`O`) opens it in a tab of its own. A version whose page declares tweaks has *Tweaks* (`t`), which shows or hides the tray beside its frame (under it, on a narrow window): its presets, a control for each tweak, *Copy*, and, once any's changed, *Compare with how it opens*. *Promote* (`p`) asks which version (the one you're on, at first), what it went into (a mission, a new one, or a document), and, the first time, where it's to live: the folder `[artifacts]` offers it in a repo, this machine only, or another folder; for one that lives in a repo, that isn't asked, and the dialog says which version is there and which takes its place. *Discard* asks why (you can leave it empty); a discarded one has *Restore*. Each says when it lands, as a commit, in the repo it lives in. Under its note, the head says what was chosen and what it went into; where it lives, and which version is there, with *Put v5 there* when the version you're on is another (one press: it's promoted, to the same folder); when it expires, in its last week; and, discarded, where its files are kept. In the strip of versions, the one that's in the repo is underlined. The frame follows the web UI's theme: an artifact that covers both schemes (`prefers-color-scheme`) shows the one you're in. `esc` goes back to the list.
- **Comparing** puts two versions side by side, each running in its own frame, at the same width (*Fit*, *1280*, *768*, *390*, `w`), with its own head: which artifact, its versions as a strip, the version's line and note, and *Open alone*. From an artifact's page, *Compare* opens on the version you're on and the one it's based on (`/artifacts/a3/compare?v=1,3`); on v1, v1 and the latest; on an artifact with one version, it beside the newest other artifact. Two artifacts compare the same way (`/artifacts/compare?a=a3.v2&b=a5.v1`), for two ideas rather than two takes on one: each side's menu puts any artifact there, at its latest. `]`/`[` step the right one's versions and `}`/`{` the left one's; *Swap* (`S`) puts each where the other was. *Source diff* (`d`) shows the left's HTML against the right's in the diff view reviews use, side by side or unified (`u`), with the unchanged lines between the changes there to open; swapped, it reads the other way about. *Reload* (`r`) and *Stop* (`x`) do both frames. Each side whose page declares tweaks has *Tweaks* in its head, which opens them over the page, and its own state in the address (`a.density=compact` for the left, `b.` for the right): so a version can be beside itself, as it opens and as it's set (`/artifacts/a3/compare?v=2,2&b.density=compact`, which *Compare with how it opens* in the tray opens). A side keeps its state while it's the same version, and *Swap* takes each side's state with it. The frames scroll apart. On a narrow window there's room for one: a toggle (`t`) says which is shown, `[`/`]` step it, and the diff is unified. `esc` goes back to the artifact.
- **The sandbox.** A version is HTML an agent wrote, run inside the page that controls your missions, so it's served from a route of its own (`/frame/a3/v2`), outside the app and its API, and framed with `sandbox="allow-scripts"`. Its scripts run, in an origin of their own: `document.cookie`, `localStorage` and the page around it throw, nothing it asks for leaves the frame (no `fetch`, no script, font or image from elsewhere, not even ws's own API), and it can't open a window, submit a form, show an `alert` or move the page. Opened alone, its policy holds it to the same. It's still signed in: only you are served it. The one thing it says to the page around it is what its tweaks are and how they're set (`postMessage`, which needs no origin): the web UI takes that from its own frame's window only, checks every part of it (names, kinds, lengths, how many) and draws it as text, and sends back nothing but what to set them to. What the sandbox can't stop is a script that loops and makes the tab slow: *Stop*.
- **`ws doctor`** says where a promoted one lives, and fails a `path` it can't use (no such repo, not a folder, no `{slug}` or `{number}`), a repo that isn't cloned, and one that takes pull requests; it warns where there's a records repo and no `[artifacts]`, so promoted ones stay on this machine, and says what to add, of a `guide` that isn't there, and of folders left in the repo from when artifacts were kept there whole; and it says how long a live one lasts. It says what's in `.ws/artifacts` that belongs to no artifact: a folder with no `artifact.toml`, a version file its `artifact.toml` doesn't name (a version is written before its record names it, so that's what a write that didn't finish left), and a file beside the folders. Anything else in an artifact's folder is the artifact's.

**Threads on a document.** A markdown artifact (a proposal, an ADR, a plan, a handoff) is commented on where you read it, as in a shared doc: open it from the mission's page, select text and press `c` (or click *Comment* beside it). From the keyboard, `j`/`k` move a cursor paragraph by paragraph (list items, table rows, code blocks and diagrams count), `⇧↓`/`⇧↑` take in the next, and `c` comments on them; a diagram is commented on whole. A comment is a thread, live as soon as you post it (`⌘⏎`): *Ask* (you want an answer), *Change* (you want the document changed) or *Note* (`tab` picks). Threads sit in the margin, level with the text they highlight, coloured by kind; `n`/`p` go from one open thread to the next, `⏎` replies, `x` resolves or reopens, and `a`/`d` accept or decline a suggested edit.

- **What's kept.** A thread keeps the text it highlights as it reads rendered, with a little either side, and the markdown lines it's in. Crew read both (`ws show t3`), so they find it in the source; the page finds the text again however the document changes around it. A thread whose text has been rewritten or removed is listed above the document, with what it said. Each version of the document a thread or reply is on is kept, in the mission's `docs/`.
- **An agent answers.** When you post an *Ask* or a *Change*, or reply in a thread, an agent answers in it, usually within a minute: the card says it's answering, and its reply appears in the thread. It starts from `ws show t3` (what the text is, the document's history and stage, who wrote it, the decisions about it) and reads what it needs, but can't change anything: for a change, it suggests an edit (see below) and says why. Each thread keeps its own session, so it remembers the thread as you go back and forth. It's Claude Code (`claude -p`, resumed per turn with `--resume`), with `--permission-mode dontAsk` and an allowlist of reading and `ws show`, on `[conversation] model` (default `sonnet`). If it can't answer, the thread says why. A *Note* on its own isn't answered.
- **Suggested edits.** A suggestion replaces text in the document's markdown, quoted exactly (it must appear there once), with what it should read. The thread agent makes one for a *Change* (`ws thread suggest t3`, as crew can too); you can make one yourself from the comment box: *Suggest an edit*, then type what the highlighted text should read (when that text isn't in the markdown as it reads, around formatting say, it's posted for the agent to make into an edit). The card shows it as before and after, and the text it replaces is struck through in the document. **Accept** (`a`, or `ws thread accept t3`) writes it into the document, and resolves the thread: in a capsule, it commits just that file, as you (*Accept suggestion on proposals/x.md (t3)*), or, if the file already has uncommitted changes (crew's draft, say), writes the edit in alongside them and commits nothing, so their changes aren't committed as yours; a mission file is just written; a document on a default branch can't be changed in place. If the text it replaces has changed since, it's refused: ask for a new one. **Decline** (`d`, `ws thread decline t3`) keeps the thread open, for you to say why. A newer suggestion replaces one still waiting.
- **Answers from crew.** Crew see a mission's open threads in `ws show` and answer them with `ws thread reply t3 "…"`, after changing the document (and committing it, when it's in a capsule) for a change. Only you resolve a thread (`ws thread resolve` refuses crew). `ws thread list` lists them.
- **Your turn.** A thread someone has answered is your turn (while the agent is answering, the mission's headline is `… answering`, in the agent's court): the document's page says how many, the mission's page shows it beside the document, and the mission's headline is `↩ replies` in your court, with `⏎` going straight to the document. What's changed since you last commented is marked with bars in the left margin; click one for what that passage said then (`i` hides them).

```toml
[web]
port = 47100    # default: one of 47000–47999 picked from the workspace's path, or the next free one

[ui]
accent = "#2a9d8f"   # the colour the web UI is drawn in (default: ws's own violet)
sidebar = ["reviews", "missions", "conversations", "x/jira/board"]   # the sidebar's places, in order
menu = ["team", "files"]                                             # moved into the workspace's menu
```

- Its colour is `[ui] accent`, a hex colour (`#rgb` or `#rrggbb`; anything else is refused): the team's in `ws.toml`, yours in `ws.local.toml` or on the Settings page, yours winning. The page is served with it on `:root` as `--accent-pick`, so it never shows the default first. Unset, nothing is added, and the web UI's own default applies. Everything accent-coloured (buttons, links, selection, the mark and the horizon) is worked out from it for each theme: lightened or darkened as far as it needs to read, the button's label white or near-black by contrast.
- The sidebar's places (Missions, Conversations… and each view a plugin adds) are yours to arrange: *Arrange the sidebar…* in Settings, under *Appearance*, turns the sidebar into two lists, the sidebar's and the menu's, each place dragged by its grip (or, from the keys, picked up with `⏎` on it, moved with the arrows and put down with `⏎`) or sent across with the arrow at its end. *Done* writes them into `ws.local.toml` as `[ui] sidebar` and `[ui] menu`, each a list of ids: ws's own by name (`missions`, `conversations`, `documents`, `artifacts`, `reviews`, `team`, `files`, `orchestrator`, `runs`), a plugin's view by its address (`x/jira`, `x/jira/board`). *Cancel* leaves it as it was, and *Reset* takes yours away. Places in the menu are at its top, with their counts; when one wants you, a dot on the workspace's name says so. A place in neither list (a plugin installed since, or a new place in ws) goes at the sidebar's end, or the menu's for one whose home is the menu (*Runs*), until you arrange it; an id that isn't a place now is kept where it is, for a plugin that's away a while. The two are one arrangement: the team's can be set in `ws.toml`, and yours, setting either, replaces it. Under the button in Settings is whose the arrangement is, with *Reset* when it's yours.
- **Focus** is what you've pinned, kept in the sidebar below its places: anything ws names (a mission, a conversation, a pull request, a page, an artifact, a document, a plugin's thing), a row each, drawn as it stands (a mission's headline, a conversation waiting on you, a pull request whose checks fail). Pin one from its card (*Pin*, beside the way to it), its page's *Pin* or `.` anywhere on its page (a mission's pages, a conversation, a pull request, a page, an artifact, a document on its space's default branch, a plugin's thing), `⌘.` on its row in Find, or by dragging a link to it (a list's row, a name in what you're reading) onto the sidebar, where it's put as it's dropped. Unpin from its row (`✕`, with *Undo*), or any of those again. Drag a row to put them in order. With nothing pinned there's no Focus at all. A pin leaves Focus by itself once what it names has ended (a mission landed, dropped or archived, a conversation resolved, an artifact discarded), and comes back with it if it's brought back; a plugin removed takes its things' pins with it. `ws pin <ref>…` pins from the command line, `ws pin` lists what's pinned, and `ws unpin <ref>…` takes one out. Pins are yours: crew and the orchestrator are refused.
- On the Settings page (*Appearance*) it's ws's own and a few others to pick from, a colour picker for any other, or its hex typed: the page changes as you pick, and it's written when you let go, with *Undo*; *Reset* (or ws's own) takes it away. Under them is the accent as the current theme shows it (in text, as a tint, on a button), and a word when the theme has had to lighten or darken your pick to read.

- It listens on `127.0.0.1` only. `ws` alone opens it, as `ws web` does. `ws web --print [path]` (or `ws --print`) prints the link instead of opening it, and `ws daemon status` says where it is.
- The link signs your browser in with the workspace's token (kept in `.ws/web.json`, readable only by you, and the same across restarts), so open it with `ws web` rather than typing the address. Every request needs that sign-in, comes to `127.0.0.1` or `localhost` by name, and, for anything that changes something, comes from the UI's own pages.
- Working on the UI: `just web-dev [workspace]` serves it with hot reload at `localhost:5173`, talking to that workspace's running web UI. `just build` builds it into `internal/web/dist`, which the binary embeds. It isn't committed, so a `go install` has no web UI. A running daemon keeps serving the UI it was built with: `ws daemon stop` (and `ws web` again) to see a new build.

---

## Configuration

### ws.toml

The shared workspace config. Lives at the root of the workspace and is typically checked into the config repo.

```toml
[workspace]
org = "your-org"              # GitHub organisation
default_branch = "main"       # default branch name for all repos
display_name = "My Workspace" # optional — shown in prompts and the web UI

[repos.frontend]
display_name = "Frontend"
aliases = ["fe", "front"]
color = "#FF6B9D"
after_create = "npm install && npm run build"

[repos.backend]
display_name = "API"
aliases = ["api", "be"]
after_create = "make setup"

[repos.infrastructure]
aliases = ["infra"]

[repos.cr-api]                 # another org's repo, whose name there is taken here
github = "cr-team/api"
```

**Workspace fields:**

| Field | Required | Description |
|---|---|---|
| `org` | yes, unless every repo says its `github` | GitHub organisation: where a repo is on GitHub (cloned from, its pull requests asked for) when it doesn't say. |
| `default_branch` | yes | The branch name used for ground (e.g. `main`, `develop`). |
| `display_name` | no | Human-friendly workspace name. Falls back to `org`. |

**Repo fields:**

| Field | Description |
|---|---|
| `display_name` | Human-friendly name shown in UI. Falls back to the canonical name. |
| `aliases` | Short names for the repo (e.g. `fe` for `frontend`). Used in commands and completions. |
| `color` | Terminal colour for this repo. Accepts hex (`#FF6B9D`) or 256-colour codes. |
| `after_create` | Shell command run in the worktree after `lift` or `dock`. Failures are logged but non-fatal. |
| `group` | The repo's [group](#groups). Ungrouped repos are always visible. |
| `landing` | How work lands: `"pr"` (the default), or `"direct"` for commits straight to the default branch with [`ws land`](#landing). |
| `verify` | A shell command that must pass in a capsule before `ws land` pushes it. |
| `github` | Where the repo is on GitHub, as `owner/name`, when that isn't `org`'s repo of its name: a repo of another org, or a person's, or one whose name there is taken here by another repo (`[repos.cr-api] github = "cr-team/api"`). The key stays its name in the workspace, in commands and refs, and its folder under `repos/`. Two repos can't be at one place. Its pull requests, review requests, links and addresses are all of where it says; an address on GitHub (`https://github.com/cr-team/api/pull/7`) is read as `cr-api#7`. `ws doctor` says of each owner whether `gh` can reach its repos (an org with SAML single sign-on needs `gh`'s token authorised for it, and until then its repos answer as though they weren't there), and of a clone that still fetches from somewhere else (the repo given its `github` after it was cloned) that it does: `--fix` points it where `github` says. |
| `documents` | The repo is a [space of documents](#spaces-of-documents) (`true`), or a folder of it is (`"docs/"`), or it isn't (`false`: the records repo is one without saying). A repo can say it of itself in its `ws.repo.toml`; `ws.local.toml` can turn one off. |

**Agents** (`[agents]`): the workspace's own way of working, for every agent `ws` starts.

| Field | Description |
|---|---|
| `guide` | The workspace's guide for agents. Unset: `AGENTS.md`, or `CLAUDE.md`, at the workspace's root, if there's one. |
| `skills` | The folder its skills are in, each a folder with a `SKILL.md`. Unset: `.agents/skills`, or `.claude/skills`, if there's one. |
| `process` | How work goes here, in prose (see *The process* under [Missions](#missions)). Unset: `PROCESS.md` at the workspace's root, beside `ws.toml`. |
| `pr` | How the workspace's pull requests are written, in prose, for the PR writer (see *The PR writer* under [Missions](#missions)). Unset: `PR.md` at the workspace's root, beside `ws.toml`. |

Every agent is told where the guide is and to read it first: crew, briefing agents, the orchestrator, and the agents that answer threads, conversations, cells and passes. Skills are listed by `ws show skills` (each one's name, what it's for from its `SKILL.md`, and that file), and in a mission's `ws show -f llm`. An agent follows a skill by reading its file, so one that can only read can still follow one, in whichever harness it runs.

Each harness also looks in a place of its own: Claude Code in `.claude/skills` and `CLAUDE.md`, others in `.agents/skills` and `AGENTS.md`. `ws doctor` says when a harness in use can't see the workspace's skills, or loads a guide that isn't the workspace's. `ws doctor --fix` links the skills, and the guide where there's none; a guide of its own that's there already is left for you to merge.

**Kinds of document** (`[documents.<name>]`, in `ws.toml` only): see *Kinds of document* under [Missions](#missions).

```toml
[documents.bp]
title = "Build proposal"
path = "records:bp/proposed/{date}-{slug}.md"
add = "docs: add bp {date} {slug}"
amend = "docs: amend bp {date} {slug}"
guide = ".agents/skills/build-proposals/SKILL.md"

[documents.bp.moves.accept]
to = "bp/accepted/"
```

| Field | Description |
|---|---|
| `title` | What one's called. Unset: the kind's name. |
| `path` | Where one goes, as `repo:path` (the repo by its name or an alias). `{date}` is the day one's started (`2026-11-11`), `{slug}` its title in lowercase words joined by hyphens, `{number}` one above the highest there, to four digits. It has `{slug}` or `{number}`, and ends `.md`. |
| `add`, `amend` | The first line of the commit that adds one, and of one that changes one, with `{date}`, `{slug}`, `{number}` and `{title}` (its first heading, or its front matter's `title`). Unset: *Edit &lt;path&gt;*. |
| `guide` | The file that says how one's written: relative to the workspace, or `repo:path` for a file on a repo's default branch. |
| `passes` | Your own passes (`[pages.passes.<name>]`) that are this kind's: tabs on pages started as one, and no others. |
| `moves.<move>` | A step in one's life: `to`, the folder it goes to (with the same name). Where one is, its path's folder or a move's, is where it stands: *proposed*, *accepted*. One in a move's folder is still of the kind (its `amend` names its changes). `commit` and `asks` are for when ws makes the move, which it doesn't yet. |

**Where a plugin's things go** (`[things.<kind>]`, in `ws.toml` only): the places one of a plugin's kinds of thing may be put when it's promoted to a repo, by the kind's name as its plugin has it (`ws plugin list`). See [Plugins](#plugins).

```toml
[things.sketch]
paths = ["records:sketches/{slug}/", "docs:architecture/{slug}/"]
subfolders = true   # one can be grouped in a folder of its place: records:sketches/cross-stack/release-flow/
```

| Field | Description |
|---|---|
| `paths` | The places, each a folder of a repo that's a [space of documents](#spaces-of-documents), as `repo:folder/`, with `{slug}` (the thing's title, as a slug) in its last part, and ending in `/`. The first is where one goes unless you say otherwise; all of them are offered as you promote one, with somewhere else still yours to name. Unset: the folder the kind's plugin names for it, in the records repo, and then in each of the workspace's other spaces. `ws doctor` fails a place ws can't use (no such repo, one that isn't a space, a path that isn't a folder for each one), and a kind no plugin here has. |
| `subfolders` | `true` lets one be grouped where it goes: as you promote it, you can name a folder of the place to put it in (`cross-stack`, or `plans/q4`), among those its kind's things are in there already, or a new one, and it lives at `records:sketches/cross-stack/release-flow/`. Unset, one goes straight in the place. Either way a folder you name whole, as somewhere else, can be anywhere in a space. |

**Artifacts** (`[artifacts]`): where a promoted artifact lives in a repo, and the commits that put it there. Every artifact is on this machine, in `.ws/artifacts`, whatever this says. See [Artifacts](#artifacts).

```toml
[artifacts]
path = "records:artifacts/{date}-{slug}/"
promote = "Keep artifact {slug}: v{version}"
guide = "records:ARTIFACTS.md"
```

| Field | Description |
|---|---|
| `path` | Where a promoted one lives, offered the first time it's promoted: a folder of a repo, as `repo:folder/name/`, the name with `{date}` and `{slug}` (or `{number}`), and ending in `/`. Unset, or `".ws"`: a promoted one stays on this machine, unless you name a folder for it (`ws artifact promote --home repo:folder/`). The repo has to take commits straight to its default branch (`landing = "direct"`): `ws doctor` fails one that takes pull requests, and ws puts none there. `ws.local.toml` can say `path = ".ws"` too, and then yours stay on this machine whatever `ws.toml` says. |
| `promote`, `discard`, `restore` | The first line of the commit that puts a promoted one's version where it lives, takes its folder away when it's discarded, and puts it back when it's restored, with `{title}`, `{slug}`, `{date}`, `{number}` and `{version}`. Unset: *Promote artifact {slug}: v{version}*, *Discard artifact {slug}* and *Restore artifact {slug}*. Each commit's body says who did it, and why or what was chosen. (`add`, `revise` and `clear`, from when artifacts were kept in a repo whole, aren't read: `ws doctor` says so.) |
| `expire_after` | How long a live one lasts after its last version (or its restoring): a number of days (`"30d"`, the default), or `"never"`. The daemon discards an expired one. |
| `guide` | A file that says how one's written, shown to a conversation's agent in place of ws's own rules (`ws artifact add --propose`, with no file): relative to the workspace, or `repo:path`. |

**Group fields** (`[groups.<name>]`):

| Field | Description |
|---|---|
| `hidden` | Leave the group's repos out of commands that cover every repo. Default `false`. |

### ws.local.toml

Per-machine overrides. Lives alongside `ws.toml` but is gitignored. Created automatically as needed.

ws writes this file as well as reading it (lifting a capsule boards it, `ws silo point` records the target, the web UI's *Settings* page sets what you choose there). It changes only the lines of the keys whose values change, so your comments, order and spacing stay, as does anything it doesn't read. A file it can't edit that way (a key in an inline table, or set as a dotted key) is written whole when ws records something itself, which loses its comments, and the Settings page says to edit it by hand instead. Writes take turns in the workspace's daemon, as every write to its records does (a command that writes the file starts the daemon if it's down), so a lift and a setting saved at once keep both.

```toml
git = "ssh"

[repos.frontend]
after_create = "pnpm install"
aliases = ["f"]

[boarded]
frontend = [".ground", "my-feature"]
backend = [".ground"]
```

**Top-level fields:**

| Field | Description |
|---|---|
| `git` | Clone protocol: `"ssh"` or `"https"` (default). |

**Team:** `[team] window`, `narrative`, `harness`, `model`, `[team.watch.<name>]` and `[team.people.<key>]` (see *Team* under [Missions](#missions)) can be set here too; a person here replaces the shared one of the same key.

**Agent overrides:** `[agents] guide`, `skills`, `process` and `pr`, and each `[crew]`, `[briefing]`, `[orchestrator]`, `[conversation]`, `[judge]` and `[pr_writer]` setting, replaces the shared value (see [Agents](#missions) under Missions), so you can run Claude Code while the team default is Cursor.

**Updates:** `[updates] check = false` stops this workspace's daemon looking, once a day, for a later release of ws and of each plugin installed from a repo (see [`ws upgrade`](#other-commands)). It's here only: it's your machine that asks.

**Artifacts:** `[artifacts] path` says where the artifacts you promote live, whatever `ws.toml` says: `".ws"` keeps them on this machine, unless you name a folder for one as you promote it. One that lives in a repo already still goes there.

**Repo overrides:**

You can override `display_name`, `color`, `after_create`, `group`, `landing`, `verify`, `github` and `aliases` per repo. Aliases are appended to the shared list; other fields replace the shared value. A repo `ws.toml` hasn't is [yours](#repos-of-your-own), whole as you say it.

**Group overrides:**

`[groups.<name>] hidden = true|false` replaces the team's setting for you. `ws group show`, `hide` and `reset` write this for you.

**Boarded section:**

Managed automatically by `ws`. You generally don't edit this by hand. Lists which capsules are currently visible in your IDE workspace.

### ws.repo.toml

Some repos need setup work before you can develop in them — installing dependencies, copying local config files, running code generation. These steps need to happen every time someone creates a capsule, and they're specific to the repo, not the workspace.

`ws.repo.toml` lives inside the repo itself, committed alongside the code. Place it at the root of the repo (it'll be read from `.ground/ws.repo.toml`).

```toml
[capsule]
copy_from_ground = [".env", "config/local.yaml"]
after_create = "npm install && npm run codegen"
```

**`copy_from_ground`** — Files that exist in your ground worktree but aren't checked into git (local config, `.env` files, generated certs). When a capsule is created, these are copied from `.ground/` into the new worktree before any hooks run. Missing files are silently skipped.

This solves a common pain point: you set up `.env` once in ground, and every capsule gets a copy automatically. No more "why isn't my app starting" after lifting a new branch.

**`after_create`** — A shell command run in the new worktree after file copying. Use it for dependency installation, build steps, or anything the repo needs to be workable.

This field is a fallback — if `after_create` is also set in `ws.toml` or `ws.local.toml` for this repo, those take precedence. The priority order is:

1. `ws.local.toml` (your machine-specific override)
2. `ws.toml` (workspace-wide config)
3. `ws.repo.toml` (repo's own default)

This lets repos ship sensible defaults while still allowing workspace-level or personal overrides.

| Field | Description |
|---|---|
| `copy_from_ground` | List of file paths to copy from `.ground/` into new capsules. Paths are relative to the repo root. Missing files are skipped. |
| `after_create` | Shell command run after capsule creation. Used as a fallback when no workspace-level hook is set. |

Above `[capsule]`, a repo can also say how work lands in it, since that's a fact about the repo and not about one workspace (see [Landing](#landing)). The workspace's `ws.toml` and `ws.local.toml` take precedence.

```toml
landing = "direct"
verify = "just lint"
```

| Field | Description |
|---|---|
| `landing` | `"direct"`: commits go straight to the default branch, with `ws land`. |
| `verify` | Shell command that must pass in a capsule before it's landed. |

---

## Shell Integration

### The ws Wrapper

`ws` commands like `lift`, `dock`, `jump`, and `mc` need to change your shell's working directory. Since a subprocess can't change its parent's directory, these commands print a `cd` command to stdout, and a shell wrapper function captures and evaluates it.

Add this to your `~/.zshrc`:

```bash
eval "$(workspace shell-init zsh)"
```

This defines a `ws` function that wraps the `workspace` binary. For `jump`, `lift`, `dock`, `init`, and `mc`, output is evaluated in your shell. All other commands pass through directly.

Without this wrapper, navigation commands will print a `cd` path instead of actually navigating.

`burn` (and `land`, which burns the capsule it lands) is handled differently. It removes a worktree that may be the very directory your shell is standing in, and a shell whose working directory has been deleted is in a bad state — some terminals kill it outright. So the wrapper moves first: it steps out to the workspace root, runs the burn from there, and returns you to where you were if the capsule survived. The original directory is passed along in `WS_ORIGIN_PWD`, so `ws burn <capsule>` still infers the repo from where you ran it. Without the wrapper, `ws burn` still works — you're just left standing in the deleted directory and have to `cd` out yourself.

### Tab Completions

The shell wrapper also sets up zsh completions. Repos, capsules, editor names, and subcommands are all completed:

- `ws lift <TAB>` — repo names and aliases
- `ws burn frontend <TAB>` — capsule names in frontend
- `ws jump <TAB>` — repo names and aliases
- `ws open <TAB>` — `cursor`, `code`, `cursor-agent`, `idea`

### Starship

Use `ws prompt` to feed workspace context into [Starship](https://starship.rs/) or any other prompt framework.

**Short format** — returns `Workspace / Repo Display Name`:

```bash
ws prompt --format short
# → My Workspace / Frontend
```

**JSON format** — returns all available context:

```bash
ws prompt --format json
# → {"workspace_display_name":"My Workspace","repo_name":"frontend","repo_display_name":"Frontend","repo_color":"#FF6B9D","capsule_name":"my-feature","is_capsule_boarded":true}
```

**Custom template** — Go template syntax:

```bash
ws prompt --template '{{.RepoDisplayName}} {{.CapsuleName}}'
# → Frontend my-feature
```

Available template fields: `WorkspaceDisplayName`, `RepoName`, `RepoDisplayName`, `RepoColor`, `CapsuleName`, `IsCapsuleBoarded`.

**Example Starship config** (`~/.config/starship.toml`):

```toml
format = '${custom.ws}$all'

[custom.ws]
command = "workspace prompt"
format = "[$output]($style) "
when = true
symbol = " "
style = "italic cyan"
```

Add `${custom.ws}` to your `format` string to position it in your prompt. `when = true` always runs the command — Starship hides the module automatically when the output is empty, which is what `ws prompt` produces outside of a workspace.

---

## IDE Integration

### VS Code / Cursor

`ws` generates a `workspace.code-workspace` file at the workspace root. This file's `folders` array is kept in sync with your boarded capsules. When you board or unboard a capsule, the file is regenerated and your editor updates its sidebar.

Each folder is named `Display Name (capsule)` — e.g. `Frontend (.ground)` or `API (my-feature)`.

The workspace file must be valid JSON (no comments or trailing commas). `ws` preserves any settings or extensions you've added; it only rewrites the `folders` key.

### IntelliJ

`ws` generates IntelliJ project configuration at the workspace root. Boarded capsules are added as content roots in the project structure.

### Opening Your Editor

```bash
ws open           # opens in Cursor (default)
ws open code      # VS Code
ws open idea      # IntelliJ IDEA
ws open cursor-agent
```

This opens the workspace-level project file, which includes all your boarded capsules.

---

## Runs

`ws run` starts a dev server, a compose stack or anything long-lived where you stand (a capsule, `.ground` or `.silo`), on a free port, and ends it for you. Every run has an **owner** and a **time**. It's ended once either has gone, so nothing is left running after the work it was for.

That's a **service**, one of two types of run. The other is a **task**: tests, lint, a build, which ends by itself and is kept for what it came to (see *Tasks*, below).

```bash
ws run -- pnpm dev --port {port}
ws run --name api --for mission --ttl 8 -- go run ./cmd/api
```

Several words after `--` are each run as a word (`ws run -- sh -c 'echo hi; exec serve'`); one is a command line of its own (`ws run -- 'pnpm build && pnpm preview'`). `{port}` in the command is replaced with the run's port, and `$PORT` is set as well. Ports come from 48000–48999, and a run started again gets the port it had if that's still free. `ws run` waits for the run to be up (`--wait`, a minute unless you say), then prints the run's name, its link and when it ends. Up means something listens on the port, or for a recipe with `ready`, that path answers below 500. The port is looked for on `127.0.0.1` and then `::1`, here and by the router, as a dev server that binds `localhost` can get `::1` alone; the run's own link is `127.0.0.1`'s. If the command ends first, it prints the end of the command's output instead.

**Recipes.** A repo says how it's run in its `ws.repo.toml`, and `ws run <name>` runs that recipe in the worktree you stand in. `ws run` alone lists them. A recipe is written in the table of the type of run it is: a service under `[run.service]`, a task under `[run.task]`.

```toml
[run.service.web]
command = "pnpm dev --port {port} --strictPort"
ready = "/"                    # up once this answers below 500

[run.service.stack]
command = "docker compose up"
stop = "docker compose down"   # run where it ran, once the run is ended
```

The two types share their names, as `ws run <name>` says a recipe by its name alone: one written as both a service and a task isn't run until one of them has another. A key that's the other type's (`ready` or `page` on a task, `reports` on a service) isn't read, and `ws doctor` and Settings say so, as they do of any key ws doesn't know. `[run.<name>]`, with no type, is how recipes were written before tasks: it isn't read now, and is said the same way.

**A recipe runs once in a worktree.** Asked for where it's running already (by you, crew, a conversation or the web UI), `ws run web` starts nothing: it gives you the run that's there, and that run is then yours as well as whoever started it. It's ended for its owners' going only once all of them have gone, and has the longest of the times they asked for. A command after `--` is no recipe's, and is started each time it's asked for.

**Tasks.** A recipe under `[run.task]` is one that ends by itself, and `--task` says the same of a command:

```toml
[run.task.test]
command = "go test -coverprofile=coverage.out ./..."
reports.coverage = { path = "coverage.out", format = "gocover" }   # a file its command writes, kept with its result
```

```bash
ws run test                    # prints what it prints, and exits as it did
ws run --task -- pnpm lint     # a command as a task
ws run show test               # what it came to, its reports, the runs before it
ws run show test#3             # an earlier run of it
```

- **No port.** A task listens on nothing: it has no port, link or name under `.localhost`, and `ready` isn't its to have.
- **Waited for.** `ws run` prints the task's output as it comes, waits for it to end, says what it came to, and exits with the task's own status, so `ws run test && …` reads as the command would. `--wait 0` starts it and returns; `--wait 5m` waits that long. Ctrl-C stops it.
- **Kept.** Once it has ended, a task is a result: whether it passed (exit 0) or failed and how it exited, how long it took, all it printed, the commit its worktree was at (and whether it had changes that weren't committed), and its reports. The last 10 runs of the same task are kept behind the latest, each by its number (`test.my-capsule.api#3`). A result is kept while the worktree it ran in is there, for 30 days at most; `ws run stop <name>`, or Clear on the Runs page, clears it and the runs behind it.
- **Reports.** `reports.<report>` in a task's table (or a table of its own, `[run.task.<name>.reports.<report>]`) names a file the command writes: `path`, from where it runs (a pattern, `junit/*.xml`, is every file it matches), and `format`, what it's written in (`gocover`, `lcov`, `cobertura`, `junit`…). As the command ends, ws copies what's there into the run's folder, under `reports/<report>/`. A file older than the run isn't copied: it's another run's. A plugin that reads the format shows it (see [Plugins](#plugins)).
- **Once in a worktree,** as a service is: asked for while it's running, `ws run test` waits for the run that's there. Asked for once it has ended, it runs again, and the last result goes behind the new one.
- **Timeout.** A task ends by itself, so one that hasn't after its timeout is stopped, at the daemon's next look (every 15 seconds): an hour, unless its recipe says another (`timeout = "10m"`, or `"1h30m"`), or `ws run --timeout 10m` does for this run. It's what catches something written as a task that was meant to be kept up, which is a service. The time runs from when the task started, and asking for it again while it runs gives it no more. `--ttl 0` is no limit.
- **Stopped.** A task still has an owner while it runs. One that's stopped before it ends (its owner went, its timeout passed, `ws run stop`) is kept as *stopped*, with why, and no result.
- **What it needs** is seen to as a service's is: a task that needs the api has it started first, and it's ended once the task has. A task can't be what a run needs.
- **The workspace's own.** A recipe in `ws.toml` can be a task as a repo's can: run at the workspace's root, said by its name alone (`docs-check`, and `docs-check#3` for one run of it), and the commit it's kept with is the root's. A task can't be an app: it ends by itself, with nothing to frame, so `page` isn't a task's to have.

**Needs.** A recipe says what it needs running first, and how it's told where that is:

```toml
[run.service.web]
command = "ng serve --port {port}"
ready = "/"
needs.api = { run = "backend/server", env = "API_URL", mount = "/api" }
```

`run` is a service of another repo, as the workspace names it (`"server"` alone is this repo's); `env`, the variable its address is handed over in, in web's environment; `mount`, a path below web's own name that it answers at (see *Names*, below). A need with a lot to say can be a table of its own instead:

```toml
[run.service.web.needs.api]
run = "backend/server"
env = "API_URL"
mount = "/api"
```

`ws run web` then sees to each need before it starts web: it finds the need's run, starts it if it isn't running, waits for it to be up (`--wait`, for each), and starts web with its address. With no `run`, a need is this repo's service of the need's own name (`needs.db = {}` is `[run.service.db]`). What a need's run needs is seen to the same way, first.

- **Where the address goes.** In the variable `env` names; and in the command, where `{api}` is the address and `{api.port}` the port on `127.0.0.1` (for something in a container, which can't use a `.localhost` name). The address is the run's name where the workspace has names, which stays the same when the run is started again, and `http://127.0.0.1:<port>` where it hasn't. It has no trailing slash.
- **Which run.** The nearest. For a recipe of the same repo, the one in the same worktree. For another repo's, the mission's capsule in that repo, if the mission has one, else that repo's `.ground`. ws says which it chose when it starts one. `--with api=<capsule>` (or `ground`, or `silo`) says another, and `--with api=https://staging.example.com` an address, behind which ws starts nothing. `--with` is for the recipe you ask for, when it's started.
- **One run, shared.** Two runs that need the same thing, in the same place, share one run of it. A run ws started for a need is for whatever needs it: it's ended once no run that needs it is running, at the daemon's next look, or at once when you stop the last of them. A run that was someone's before another needed it isn't ended while that one runs, whoever it's for and whatever its time. ws counts what needs a run afresh each time it looks, from the runs that are there, so nothing's left running by a count that went wrong.
- **Asked for again.** `ws run web` where web is running starts again whatever it needs that has ended. Where that comes back at another address (no names, and its port taken meanwhile), ws says so: web has to be started again to be given it.
- A run started for a need isn't the asker's: it's started without the mission, crew or conversation the asker is.
- **A folder, not a run.** `repo = "…"` in place of `run` is a need of one of that repo's worktrees, not of a run: chosen as a run's would be (the mission's capsule there, else the ground, or `--with`), and handed over in `env` and as `{name}` in the command, as its path. Nothing's started for it. This is for what's built from another repo's files (an API reference from its spec), not served by it. A run started on a folder that has gone since (its capsule burned) is ended.

**The workspace's own runs.** What belongs to no one repo (API docs drawn from two repos' specs, a mail catcher every capsule sends to) is a recipe of the workspace's, in `ws.toml`, run once for the whole workspace at its root. `ws run docs`, anywhere in the workspace but its repos, runs it; `ws run` there lists them. It's said by its name alone (`docs`), and named `docs.<workspace>.localhost` where the workspace has names.

```toml
# ws.toml
[run.service.docs]
command = "uv run tsp-serve.py --port {port} --spec BFF={bff}/spec/main.tsp --spec API={api}/packages/api-http"
ready = "/"
needs.bff = { repo = "bff" }   # bff's folder: its ground until it's pointed elsewhere
needs.api = { repo = "api" }
```

A workspace recipe is asked for from nowhere in particular, so its needs aren't the nearest to where it was asked: each is **pointed** at one of its repo's worktrees, and is met by the repo's ground until it is. `ws run point docs` says what each need is pointed at and could be (the ground, the silo, and every capsule of the repo, or for a run, those with its recipe); `ws run point docs bff=my-capsule` points one (`ground`, `silo`, or `nearest` to go back). A pointer is kept, and outlasts the run. Pointing a need of a recipe that's running starts it again on what it's pointed at now, for whoever it was for and with the time it had left. A need pointed at a capsule that's gone since is met by the nearest, which `ws run` says. Pointing is yours: crew and agents are refused, as it changes what's shown to whoever has it open. A need that's another of the workspace's recipes (`run = "mailpit"`) can't be pointed: there's only the one.

**Apps.** A service with a `page` is an app, which shows its run in a frame. A repo's (in its `ws.repo.toml`) is a tab of each of its worktrees, in Repos and on a mission's capsule's files: run in that worktree, with what it needs the nearest to it (another repo's folder from the mission's capsule there, else its ground), so a capsule's API docs are of that capsule, and of the mission's other capsules beside it. The workspace's own (in `ws.toml`) is a place of its own in the web UI's sidebar (and in Find), for what isn't any worktree's: a mail catcher, a database's UI, docs drawn from what its needs are pointed at. It's a way of adding a tool to ws that's any web server at all, with nothing of ws's to learn: API docs, Storybook, a mail catcher, a database's UI. (A plugin is the step on, for what has to know ws's things.)

```toml
[run.service.docs]
command = "uv run tsp-serve.py --port {port} …"
ready = "/"
page = { title = "API docs", icon = "book" }   # icon: one of the web UI's, by name; a box unsaid
```

Nothing runs until it's opened. Opening it starts the run, with what it needs, or joins the one that's running; the page says it's still open every minute while it's in view, and the run is ended 15 minutes after nobody has it open (it's owned by whoever's looking, as a run started from a mission's page is). It's laid out as a sketch is: a bar, and the app under it as a card, with the sidebar folded to a rail. The bar has, for a workspace app, a dropdown for each of its needs that can be pointed (choosing one starts it again on that, as `ws run point` does), and for a worktree's, what met each need; how it stands (starting, up, or how it ended, with the end of its output while there's nothing to show), and Start again, Stop and a link to open it in a tab of its own. Stopped there, it isn't started again until you say. It's shown once it answers where the page frames it: at its name, once the router has its route, where the workspace has names, or at `127.0.0.1` and its port. Most apps draw themselves light: one in a dark theme is framed as it draws itself.

In the web UI, a capsule on a mission's Work tab lists its runs (each with its link, what it needs, when it ends, and Stop) and a **Run** button for each recipe that isn't running. A run started there lasts while someone has the mission open, and 15 minutes after, or 4 hours at most; what it needs is started with it. A task there shows what it came to and the commit it ran at, and goes to its run on the Runs page; one started there is the mission's, and runs to its end, or its timeout, whether or not the page stays open.

The **Repos** page does the same for any repo's ground or capsule, mission or not: its *Runs* tab has the runs in it, whoever started them, and a **Run** button for each of its recipes that isn't running (not a silo's). A run started there lasts while someone has that ground or capsule open on the page, and 15 minutes after, or 4 hours at most. A task there shows what it came to and goes to its run on the Runs page; one started there runs to its end, or its timeout, whether or not the page stays open.

The web UI's **Runs** page (`/runs`, in the workspace's menu until you arrange it into the sidebar, and in Find) is every run in the workspace, whoever started it: each with its link, where it runs and who it's for (going to the mission, crew or conversation; or the runs that need it), what it needs, when it ends, its output (opened under it, and read again every 2 seconds while it runs), and Stop. Tasks that have ended are its **Results**: each with what it came to, the commit it ran at, the check of a mission's step it was run for, and its reports; opened, the runs of it that are kept, each with all it printed. `/runs?show=<name>` opens one. Services that ended by themselves are below, each with Clear. Above them it says where the workspace's names stand: its web UI's name, the port the router's on and which daemon holds it, or, with names off, how to turn them on.

`stop` is given a minute, with `$PORT` set. If the place it ran has gone (the capsule was burned), it can't be run, and the daemon's log says so.

**Owners** (`--for`):

| `--for` | Ends when | Default for |
|---|---|---|
| `pid` | the process that started it exits: your terminal's shell, or the agent whose command it is (the nearest above `ws` that's been running a minute or more; `--owner <pid>` names another) | everyone else |
| `capsule` | the capsule it runs in is burned or landed (not for the workspace's own runs) | |
| `mission` | the mission (`$WS_MISSION`, or the capsule's) ends | |
| `step` | the step of the plan crew are on passes or is skipped | crew |
| `crew` | crew's session closes | crew on no step |
| `conversation` | the conversation (`$WS_CONVERSATION`) is resolved | a conversation's agent |
| `none` | only its time, or `ws run stop` | |

**Time** (`--ttl`): hours, 4 unless you say. `0` is no limit, which crew can't have. `ws run keep <name> [hours]` gives a running run more time from now.

**Names.** A run is named for its recipe, its command's first word, or `--name`, and is said as `<name>.<worktree>.<repo>` everywhere (`web.my-capsule.api`, `web.ground.api`). If a command's run in the same place is running with that name already, the new one gets the next free name (`web-2`) and ws says so. A run that has ended gives its name up.

```bash
ws run ls [--json]          # every run in the workspace, ending any that are due first
ws run show <name> [--json] # where a run stands: for a task, what it came to, its reports and the runs before it
ws run logs <name> [-f]     # its output; -f follows it until it ends
ws run stop [name...]       # end runs now: those named, or every run where you stand (--all: every one), and what was started for them that nothing needs now. A task that has ended is cleared
ws run keep <name> [hours]  # more time
```

**What ends them.** The workspace's daemon looks every 15 seconds, as it starts, and at once when crew end. What fell due while it was down is ended when it's back. `ws run` starts the daemon if it isn't up. A run isn't the daemon's child: it leads a session of its own, so restarting or upgrading ws leaves it running. Ending a run asks its process group to stop (`SIGTERM`), gives it 5 seconds, then makes it (`SIGKILL`). ws only ends runs it started. A run that a running run needs isn't ended by the daemon, only by `ws run stop` or Stop, which say what needed it; and a run whose capsule has gone is ended, needed or not.

A service whose command exits by itself is kept for an hour, with its status and output (`ws run ls` shows `exited 1, 14:05`), then removed. A task is kept for what it came to (see *Tasks*).

**Names.** With `[router] enabled = true` in `ws.local.toml` (Settings › Runs › Local names, which also sets `[router] name`), the workspace and its runs have names under `.localhost`, which browsers send to this machine by themselves:

| Name | Goes to |
|---|---|
| `<workspace>.localhost` | the web UI (`ws web` signs in there) |
| `<run>.<capsule>.<repo>.<workspace>.localhost` | a run in a capsule |
| `<run>.ground.<repo>.<workspace>.localhost` | a run in a repo's `.ground` |
| `<run>.silo.<repo>.<workspace>.localhost` | a run in a repo's silo |
| `<run>.<workspace>.localhost` | one of the workspace's own runs |
| `<run>.<capsule>.<repo>.<workspace>.localhost/api/…` | what the run needs, mounted at `/api` |

`<workspace>` is `[router] name`, or the workspace's folder's name. Every name has its own cookies and storage, so two capsules' logins don't overwrite each other as two ports' do. Links ws prints and shows use names while they're served; `ws run` prints the `127.0.0.1` address too.

One router serves every workspace on the machine, with no process of its own. Each daemon with names on tries to listen on port 80 (or 7080, where 80 is another program's or the system keeps it for root, as Linux does), and the one the system lets is the router. If it goes, another takes over within 2 seconds. Each daemon writes its routes to `~/.local/state/ws/routes/` (or under `$XDG_STATE_HOME`). The router listens on every interface, as macOS lets a user have port 80 no other way, and answers this machine alone: a request from anywhere else, or for a name not under `.localhost`, is refused. Where two workspaces ask for the same name, the first keeps it and the other has four letters of its own after it (`shop-15a4`), which it keeps while its daemon runs. `ws router` says where names are served and what this workspace's are.

A need with a `mount` answers below the run's own name, at that path and everything under it: `web.my-capsule.shop.localhost/api/users` goes to the api, and the rest of the name to web. To the browser they're one origin, as they are behind an ingress, so nothing has to allow another origin and web needs no proxy of its own. The api is asked for the path as it came (`/api/users`); `strip = true` takes the mount off first (`/users`). The longest mount a path falls under has it. A need met by an address outside ws (`--with`) is mounted too, asked by that address's own name. A mount is the router's: at `127.0.0.1` and web's port, web answers everything itself. Where what's mounted isn't running, its path says so.

A recipe with `where = "silo"` runs in the repo's silo, whichever capsule it's pointed at, as `<run>.silo.<repo>.<workspace>.localhost`. Asking for it by a capsule's name (`stack.my-capsule.api.shop.localhost`) goes straight through when the silo is on that capsule. When it's on another, a page offers to point the silo there, which you confirm in the web UI; a request alone never moves it.

**Where it's kept.** Each run is a folder, `.ws/runs/<repo>/<worktree>/<name>/` (the workspace's own, `.ws/runs/@workspace/@/<name>/`), holding `run.json` (its command, port, process, who it's for, what it needs and its time) and `out.log`; a task's also holds `exit` (its status) and `reports/`. The earlier runs of a task are in `.ws/runs/.past/<repo>/<worktree>/<name>/<n>/`, each a folder of the same. Nothing about a run is kept anywhere else, but what the workspace's recipes' needs are pointed at, in `.ws/runs/@workspace/points.json`.

## Silos

### The Problem

Some tools expect a single, stable directory path. Docker compose, for example, needs volume mounts that don't change when you switch what you're working on. With git worktrees, every capsule has its own directory, so you'd have to reconfigure ports, mounts, and container setups every time you want to test a different branch.

### How It Works

A **silo** is a per-repo directory (`.silo/`) that acts as a stable runtime location. You point it at any capsule, and the workspace's daemon mirrors the capsule's files into `.silo/` as they change, with no terminal kept open for it. Docker compose always runs from `.silo/` — you switch what's being tested by re-pointing the silo, not by reconfiguring containers.

```
repos/frontend/
  .bare/
  .ground/          # default branch (sacred)
  .silo/            # stable runtime mirror
  my-feature/       # capsule
  bugfix-login/     # capsule
```

Each repo can have at most one silo. `.silo/` is a real git worktree (with a detached HEAD), so path-dependent tools work normally. Non-tracked files in `.silo/` — `node_modules`, build artifacts, `.env` — are left alone and belong to the silo.

Silo state is stored in `ws.local.toml`:

```toml
[silo]
frontend = ".ground"
backend = "add-health"
```

### Silo Commands

**Point a silo at a capsule:**

```bash
ws silo point <repo> <capsule>
```

Creates `.silo/` if it doesn't exist, syncs the capsule's files into it, and runs hooks. The daemon keeps it from then on (it's started if it isn't running). `.ground` is a valid target — use it to run the main branch version.

```bash
ws silo point frontend .ground     # use main branch
ws silo point frontend my-feature  # switch to a capsule
```

The web UI points one too: ws's router offers to when you ask for a silo's run by another capsule's name (see [Runs](#runs)).

**Show silo state:**

```bash
ws silo status
```

Shows each active silo, its target and how the daemon's keeping it: the folders it watches and when it last synced, or why it can't (the capsule's gone, there's no `.silo/`), and a hook that failed. The web UI's *Runs* page shows the same, with each hook's output and a *Resync* button.

**Watch it sync:**

```bash
ws silo watch
```

Shows the daemon's syncs into every silo as they're made, and its hooks. `r` asks for a full resync of each. Closing it changes nothing: the silos are kept either way. `-v` prints a line per sync instead, for a log.

**Remove a silo:**

```bash
ws silo stop <repo>
```

Removes the `.silo/` worktree and clears the silo config.

### How the Daemon Keeps Them

- **What's synced:** the capsule's tracked files, and its untracked ones that aren't ignored. Ignored ones (dependencies, build output) aren't, and nor are their folders watched: on macOS every file in a watched folder holds a file open, so a `target/` or `.venv/` would cost thousands.
- **As files change:** rapid edits are batched (200ms). A folder made in the capsule is watched, and the files already in it synced.
- **When its HEAD moves** (a checkout, pull or rebase in the capsule): a full sync.
- **As the daemon starts** (after an upgrade or a restart): a full sync of each silo, taking whatever changed while it was down. A full sync writes only the files that differ, so one that finds nothing changed touches nothing a dev server would reload for, and runs no hook.
- **When `ws.local.toml` points it elsewhere,** however that happened (`ws silo point`, `ws burn`, `ws land`, an edit by hand): it's given the new target's files, and its hooks run, unless `ws silo point` has already done both.

The daemon's log (`.ws/daemon.log`) says what it does with each, on lines starting `silos:`.

### Silo Hooks

When a silo is created or re-pointed, two hooks run in sequence (`ws silo point` runs them where you can see them; otherwise the daemon runs them, and its last hook's output is on the *Runs* page):

1. **`after_create`** — the same hook that runs when creating a capsule (precedence: `ws.local.toml` > `ws.toml` > `ws.repo.toml`). Use it for dependency installation.
2. **`after_switch`** — a silo-specific hook defined in `ws.repo.toml`. Use it for restarting services.

Both hooks run with the working directory set to `.silo/`.

### ws.repo.toml Silo Config

Repos can define silo-specific behaviour in `ws.repo.toml`:

```toml
[silo]
after_switch = "docker compose restart api"
after_change = "make generate"
```

| Field | Description |
|---|---|
| `after_switch` | Shell command run in `.silo/` after the sync target changes. Useful for restarting services that don't hot-reload. |
| `after_change` | Shell command run in `.silo/` after each sync that changed something (debounced). Useful for code generation or build steps that need to run on every file change. One runs at a time: changes made while it runs have it run once more after. |

### Integration with Other Commands

Silos integrate with the rest of `ws`:

- **`ws burn`** — warns if the capsule you're burning is an active silo target and offers to repoint to `.ground`.
- **`ws land`** — a silo pointed at the capsule it lands is repointed to `.ground`, which now has its work.
- **`ws debrief`** — if a debriefed capsule was a silo target, automatically repoints to `.ground`.
- **`ws status`** — shows a silo indicator for repos with an active silo.
- **`ws doctor`** — checks for missing silo targets and orphaned `.silo/` directories, and for a `.silo.lock` left by an older ws's `ws silo watch` (which synced the silos itself): one still running syncs them as the daemon does, so stop it.

---

## Customisation

### Repo Colors

Each repo can have a custom terminal colour, used in status output and the web UI. Set it in `ws.toml` or override it per-machine in `ws.local.toml`:

```toml
[repos.frontend]
color = "#FF6B9D"
```

Accepts hex colours or 256-colour codes.

### Post-Create Hooks

A `after_create` hook runs in the new worktree after every `lift` or `dock`. Use it for dependency installation, code generation, or build steps:

```toml
[repos.frontend]
after_create = "npm install && npm run build"

[repos.backend]
after_create = "make setup"
```

Hook failures are logged to stderr but don't block the command. You can override a repo's hook locally in `ws.local.toml`.

### Fuzzy Matching

Repo and capsule arguments use subsequence matching — every character in your input must appear in order in the target, case-insensitive. This works like fzf:

- `fe` matches `frontend`
- `bk` matches `backend`
- `mf` matches `my-feature`

If there's exactly one match, it's used directly. Multiple matches show an interactive picker. Zero matches return an error.

Aliases are included in fuzzy matching, so if `frontend` has alias `fe`, then `f` matches both the canonical name and the alias (deduplicating to one result).

### JSON Output

Several commands support JSON output for scripting and automation:

**Status:**

```bash
ws status --format json
```

Returns the full workspace state — repos, worktrees, dirty status, ahead/behind counts.

**Prompt:**

```bash
ws prompt --format json
```

Returns current workspace context as a JSON object. Useful for building custom integrations.

---

## Plugins

A plugin is a program that adds to the workspace without being part of ws: a view in the web UI's sidebar, commands after `ws`, names for its things, and a guide that tells agents the commands are there. The workspace's daemon runs each plugin as one process, started when it's first needed and kept until the daemon stops. Plugins are written with [the plugin framework](https://github.com/withworkspace/plugin), a Go package.

A plugin on disk is a folder with a `plugin.toml` (its name, its version, and the command that starts it) and an executable. What it adds, it says itself when it starts.

```bash
ws plugin add withworkspace/plugin-plugin-dev   # install a published plugin: its latest release
ws plugin add acme/plugin-jira@v0.3.0  # a release that isn't the latest
ws plugin update                  # fetch newer releases of the plugins installed from a repo
ws plugin add ./plugin-hello          # install the plugin in a folder, and start it
ws plugin add ./plugin-hello --link   # run it from the folder itself: for one you're writing
ws plugin add ./plugin-hello --link --build   # build it first: "run what I just changed"
ws plugin check ./plugin-hello    # what it registers, and what's wrong with it
ws plugin list                    # the workspace's plugins, and where each stands
ws plugin config hello            # what it can be told, and what's set
ws plugin config hello greeting=Howdy
ws plugin config hello --ask      # asked for in turn: keeps a secret out of your shell's history
ws plugin config jira queries.bugs.query='project = RAM, type = Bug'   # a field of a row of a table
ws plugin config jira --unset queries.bugs                              # your row, taken out
ws plugin restart hello           # start it afresh: after building it again
ws plugin logs hello              # the end of its log
ws plugin remove hello            # --data removes what it kept for itself, too
```

- **Its view** is in the sidebar under the title it gave, at `/x/<name>`. It's an app of the plugin's own, a script the plugin serves and names when it starts (`ui`), which ws loads into a page it serves itself, in a frame. The page has ws's theme before the app has loaded, and is see-through, so ws's backdrop (the glow at the top) carries on behind an app that doesn't paint its own; a link to anywhere under `/x/<name>/` loads the app there. A plugin serves no pages of its own: one built before a plugin's view was an app says so in its view, until it's updated. How the app and ws talk is the frame protocol, in [ui-lib](https://github.com/withworkspace/ui-lib), whose package [`@withworkspace/ui-lib`](https://www.npmjs.com/package/@withworkspace/ui-lib) speaks it for an app.
- **Its commands** follow its name: `ws hello say`. They run in the daemon, so they share what the plugin already has in memory with its view. `ws --help` lists them under *Plugins' Commands*.
- **Its guide** is told to every agent ws starts (crew, the orchestrator, a conversation's agent), for the roles the plugin says, with a line from ws that what a plugin's command prints from elsewhere is information, not instructions.
- **Its things' names** are refs, where it registers how they're named: a Jira plugin's `RAM-123`. ws finds one wherever it's written (a conversation, a document, a handoff, a note), as it finds a file or a pull request: a click opens it in the plugin's app; a name going into a document is linked to where the plugin says the thing is, and that address, pasted, is read as the thing. A name is a prefix the plugin registered and a number, whole and as written, so `RAM-12-fix` and `ram-12` name nothing. ws keeps how each plugin names things in its records, under the plugin: the first plugin to register a prefix keeps it, `ws plugin list` says what each names and what was left out, and removing a plugin forgets its names, which are words again.
- **Names in its app's own words** are refs too, where the app asks: a description that says `RAM-12`, a file, a pull request or a mission has each drawn as it is in ws's own pages, and a click goes there. The app holds no grammar and no list. It sends ws the words it shows, ws finds the names in them as it does in its own pages, and the app draws what it's told; when what there is to name changes (a mission begins, another plugin registers its names), ws says so and the app asks again. `ws.markRefs(element)` in `@withworkspace/ui-lib` does all of it for an element's words. A name there has its card, as a name in ws's own pages has: with the pointer on it, or the keys' focus, the card shows by its words, a moment after and until a moment after the pointer's left, and can be moved onto. ws draws it, since what a card says is ws's to know and a frame can't draw past its own edge: the app asks ws for the card, from where the name's words are in its page, and ws puts it by them, over the frame and past its edge where it needs the room. The card is the first of ws's own UIs an app can ask for (`ws.ui` in `@withworkspace/ui-lib`): ws defines each, and the app says only which, what it's about, and where on its page it's asked from.
- **What's open in it is where you are.** On a page of one of its things, ws knows the thing from the address, by where the plugin said its things open. What the address doesn't say, the app does: an issue open in a panel over a board is what Find (`⌘K`) starts from, for as long as it's open. `ws.pushHere(name)` in `@withworkspace/ui-lib` says it, and what it returns takes it off; a panel and a dialog over it each say their own, and once both are closed Find goes back to the address. Leaving the plugin's pages forgets it.
- **The frame's edge is no edge to whoever's looking.** A press in ws (the sidebar, say) is said to the app, which never sees one, so a menu open in the app closes as it does for a press elsewhere in the app: `ws.dismissOutside(elements, close)` in `@withworkspace/ui-lib` hears both. And a dialog's backdrop in the app, drawn with the class `ws-backdrop`, which ws's page styles, dims ws's side too: the sidebar (on a narrow window, the bar across the top) dims with the frame, in the same scrim and the same moment, for as long as the backdrop is there. Nothing in what's dimmed can be pressed, and a press on it takes no focus from the app's dialog; ws's keys and back still work.
- **Its things are data ws draws.** A plugin says what one of its things is made of (a title, where it stands, a few fields, what it says), what it's found by, and how it's drawn, as data: ws draws a thing's card under its name, and a fuller pane beside the list in Find, from what the plugin declared, in ws's own styles. Nothing of the plugin's runs there; its app, in its own page, is for what's more than looking. `ws show RAM-123` prints one by what its fields are, what it says at length marked as written outside the workspace, with what's been said of it here.
- **Its things are found**, where it tells ws of them: a plugin hands ws the things it has, and ws keeps them in its records with what it records itself. `ws find` and Find in the web UI (⌘K) then find one by what the plugin said it's found by, or by its name, and draw it, whether the plugin's running or not; `ws find` marks what it says as written outside the workspace. A name typed whole in Find goes to the thing even if ws wasn't told of it: the plugin's asked for that one, as it is for `ws show`, which wants a thing as it is now. They're kept under the plugin, as its names are: removing the plugin forgets them, and so does the plugin registering names that no longer name them.
- **Its things can live in a repo**, where it says a kind's can: a folder each in one of the workspace's [spaces](#spaces-of-documents), for everyone with the repo, holding the thing's own file, whatever the plugin makes from it (a picture), and a `thing.toml`, which is ws's:

  ```toml
  kind    = "sketch"
  title   = "Release flow"
  main    = "release-flow.excalidraw"
  picture = "release-flow.svg"
  ```

  ws finds one by its `thing.toml` and names it by its folder, as it names any folder: `records/sketches/release-flow/`, or `records:sketches/release-flow/`. The plugin mints no name for it. ws calls it what its `thing.toml` says and asks the plugin what its file is found by, once for each version of the thing (its folder, as git names it: another when any file in it changes), so it's found (`ws find`, Find), drawn and shown (`ws show records:sketches/release-flow/`) as the plugin's other things are, opens in the plugin's app, and a mission can be about it. A plugin that asks which there are is told where each is among its kind's things: what ws calls its repo, and the folders it's grouped in under the place the kind's things go (`cross-stack`, `plans/q4`; the folders from the repo's top, for one in none of the places). So a plugin lists its things as they're organised in their repos, though it's the workspace that says where they go. It's as it is on the repo's default branch, as last pulled. A thing of a kind no plugin here has is left in its space, and read when a plugin that has the kind is added; where two plugins have a kind by one name, the first has it. What a plugin does to its own things (its kind's actions in Find) isn't offered for one in a repo.
- **ws puts one there, and saves it, for you.** Promoting one of a plugin's things puts it in a repo: ws asks the plugin for it as its folder is to hold it, names the folder (the first of the places its kind's things go: those `[things.<kind>] paths` names in [`ws.toml`](#wstoml), or the folder the kind names, in the records repo; called by the title's slug, with `-2` and so on where that's taken; or where you say, as `repo:folder/`, in any repo that's a space), writes it with its `thing.toml`, commits it as you and lands it: *Add sketch: Release flow*. A thing's folder is the thing's alone: nothing's put inside another's. A repo that takes pull requests has it left in a capsule, for you to open one from, and ws finds the thing once that's merged. Saving one that's there is one commit of the folder as the plugin has the thing now (*Amend sketch release-flow*), and nothing where it's as it was. Where it changed in the repo since the plugin last had it (someone else saved it), ws gives the plugin what's there first, and the plugin brings the two together however its kind merges. Both are asked for from the plugin's own page, by a button of its own: ws shows its own UI over the page, by the button (where it'll live, to pick from where its kind's things go in more places than one, with a folder of the place to group it in where `subfolders` lets it, and *Promote*; how the save went), the same for every plugin. A plugin's command may ask for either too (`ws sketch promote sketch-12`). ws keeps that the plugin's own name for the thing (`sketch-12`) is the thing in the repo now.
- **Its things link to one another**, where a field of theirs names others: an issue's parent, what it's beside. ws keeps each as a link in its graph, by how the plugin says it points, and draws the name as what it names. `ws show RAM-123` lists what it names, each a ref to show next, and what points at it: what's in it, what names it, and the missions about it.
- **A mission is about a thing** when it says so: `ws mission new "…" --about RAM-123`, `ws mission about RAM-123` later, or *Start a mission about it* on the thing in Find, which makes one to brief, asked for as the thing's name and what it's called. The mission's page says what it's about, an agent reading the mission is told to read it, the thing says the mission's about it, and a plugin can read which missions are about its things.
- **What it can do is in Find.** A plugin's own actions are among what there is to do there (⌘K, then `>`). What can be done to one of its things is offered for the row the cursor's on: ⌘K again, or the buttons under the thing beside the list. One that needs something first asks for it there, a pick from a list or a line typed, and Escape steps back. It runs in the plugin, which tells ws of the thing as it now is.
- **Its config** is yours where you set it: `ws plugin config` writes `[plugin.<name>]` in `ws.local.toml`, over the team's in `ws.toml`. A secret is in neither file: it's kept beside the plugin's data, readable only by you. A plugin that requires something that isn't set says so in its view, in `ws plugin list`, and when its commands are run.
- **Its config in tables** is named rows, each with the same fields, where the plugin declares a key as one: a board's queries, the projects it knows. A row is a table under the key, named by the row (`[plugin.jira.queries.bugs]`), and a field of it is set by its path: `ws plugin config jira queries.bugs.label=Bugs`, a part with a dot or a space in it quoted (`spaces.ram.custom_fields."Story Points".card=true`). `ws plugin config jira` lists the rows in the order they were written, the team's first, which is the order the plugin's given them in. Yours is laid over the team's row by row, then field by field: your `queries.bugs.label` changes that row's label and leaves the rest of it, and the team's other rows, as they are. A team's row is left out for you with `<row>.enabled=false`. ws checks each row against what the plugin declared, and says what's wrong, with where, under the rows and in its log: a field that isn't declared and a value of the wrong type are left out, and a row without a field it needs isn't given to the plugin. `--ask` asks for a table a row at a time: add one, change one, take one out.
- **Where it's kept:** `.ws/plugins/<name>/`, with the plugin itself in `app/` and what it keeps for itself (its database, its log) in `data/`.

**Published plugins.** A plugin is published as a repo on GitHub, and each release of it holds a build for each platform (`<name>_<tag>_<os>_<arch>.tar.gz`: its `plugin.toml` and what its command starts) and a `checksums.txt`. `ws plugin add owner/repo` downloads this machine's build of the latest release, checks it against the release's checksums, and installs it; `ws plugin update` fetches a newer one, keeping the plugin's config and data. A repo with no release, or none built for this machine, is installed from its source, which is built here (so it needs what the plugin's built with: Go, for one written with the plugin framework). With `gh` logged in, a private repo is read as you; without it, a public one is read as anyone. `ws plugin list` says where each came from.

A plugin that hasn't been built is built when it's added, as its `plugin.toml` says (`build`), so one can be added straight from its source.

**Plugins, in Settings** (`/settings/plugins`, and in Find) is the workspace's plugins in the web UI, and does what `ws plugin add`, `update` and `remove` do.

- **Each plugin is a row:** its name and version, where it stands (running, not running until it's next needed, needs setting up, failed), what it says it is, where it's from (the repo and the release, built here from its source, a folder it was copied from, or the folder it's linked to), and what it adds: its views, its commands, a guide for agents, the names of its things. One that needs setting up says which keys, with the command that asks for them (`ws plugin config <name> --ask`); one that failed says why, with the command for its log.
- **Add a plugin** takes what `ws plugin add` does: `owner/repo` (`owner/repo@v1.2.0` for a release that isn't the latest), or a folder on this machine, by its whole path or from `~`. It's installed and started, and the row says where it stands. Running one from its folder as you write it (`--link`) stays the terminal's, and the plugin-dev plugin's.
- **Update** is on a plugin installed from a repo. Where a later release is known to be out the row says so, with a link to it, and the button names it (*Update to v0.4.0*). Otherwise it asks the repo now, and fetches a later release if there is one. One built here from its source is fetched and built again. Its config and data stay.
- **Remove** asks first, and whether what the plugin kept for itself (its data and its secrets) goes too: left alone, adding it again picks up where it was.
- **The team's that aren't here** (`[plugins]` in `ws.toml`) are listed under the installed ones, each with *Add*.
- **What's out** is what the daemon last found: it asks each plugin's repo for its latest release once a day, with ws's own (see [`ws upgrade`](#other-commands)). *Check for updates* asks now. Nothing's updated by looking.

What didn't work is said above the list, with what a build printed where it got that far.

**Writing one.** The [plugin-dev](https://github.com/withworkspace/plugin-plugin-dev) plugin does it (`ws plugin add withworkspace/plugin-plugin-dev`): `ws plugin-dev new notes` writes a plugin that works as it's written, with the workflow that publishes its releases, and `ws plugin-dev watch`, run in its folder, builds it and runs it in the workspace on every save. It's a plugin itself, and uses what's above: `ws plugin add --link --build` to run what it built, and `ws plugin check` to say what's wrong with one. `ws plugin check` starts the plugin against a workspace of its own with nothing in it, so nothing of yours is touched. It asks the plugin for its app's script and stylesheet as a browser would, and fails one whose app isn't there, isn't served as what it is, or that serves a page of its own where its view is.

A plugin is a program that runs as you. Install one as you would any other.

---

## Other Commands

A few commands that don't fit neatly into the concepts above:

**`ws doctor`** — runs health checks on your workspace (`--reindex` first has every store say its words again, for [`ws find`](#finding-whats-been-said)). Verifies repos are cloned, worktrees are intact, and required tools are available, and that the harnesses in use see the workspace's guide and skills (see `[agents]` under [ws.toml](#wstoml)). It also names anything in `ws.toml`, `ws.local.toml` or a repo's `ws.repo.toml` that ws doesn't read (a misspelt key such as `alias` for `aliases`, or one in the wrong file), which is otherwise passed over without a word.

**`ws sync`** — pulls the latest `ws.toml` from the config repo and clones any newly added repos. Reports what changed. (It was `ws upgrade`, which now upgrades ws itself.)

**`ws upgrade`** — installs the latest release of ws in place of the one that's running: its build for this machine, checked against the release's checksums and run once before it's put in place. `ws upgrade v1.4.0` installs that release, an older one or a prerelease among them; `--check` says what the latest is and installs nothing.

- **Only a release's build is upgraded** (one installed by `scripts/install.sh`). A ws built from a clone, or by `go install`, is changed where it was built, and `ws upgrade` says so. There are releases for macOS on Apple silicon and Linux (amd64, arm64).
- **The ws it replaces is kept** beside it, and `ws upgrade --rollback` puts it back. A workspace's records are moved on by the first newer ws that opens them, and an older ws won't open them after: `.ws/backup` has the day's copy.
- **The daemon follows.** A workspace's daemon keeps the ws it started from, finds the new one in its place and starts again on it once nothing's at work (see *The daemon* under [Missions](#missions)). `ws upgrade` says what this workspace's daemon is waiting for.
- **It's yours to run.** Crew and the orchestrator are refused: it changes the ws every crew and the daemon run.
- **The daemon looks for you.** Once a day a workspace's daemon asks GitHub for ws's latest release (and for the latest of each plugin installed from a repo: see [Plugins](#plugins)). It only looks: nothing's installed. A later release is said in the web UI: a line in the workspace's menu (*ws v1.4.0 is out*), and on Settings, under *About*, where *Upgrade* installs it as `ws upgrade` does, and the page says what the daemon's waiting for before it restarts onto it. A ws built from a clone is told what's out, and to build it again where it was built. *Check now* looks at once. `[updates] check = false` in `ws.local.toml` (the switch beside it) stops the daemon looking by itself: it's yours alone, as it's your machine that asks. A look that got no answer (no network) is tried again within the hour, and what was found is kept in `.ws/cache/updates.json`, so a daemon that's started again doesn't look again that day.

**`ws status`** — shows all repos and their worktrees with git status. Use `--format json` for machine-readable output.

**`ws jump`** (alias `j`) — navigate to any capsule. Supports fuzzy matching and interactive pickers:

```bash
ws jump frontend my-feature   # direct
ws jump fe mf                 # fuzzy
ws jump frontend              # pick a capsule
ws jump                       # pick a repo, then a capsule
ws jump ~                     # workspace root
```

**`ws root`** — prints the path of the workspace root, for use in scripts and shell substitutions:

```bash
cd "$(ws root)"
```
