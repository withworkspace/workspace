# ws

For agents working on ws itself. (`FOR_LLMS.md` is the guide for an agent working *in* a workspace ws manages.)

**ws runs multi-repo work.** It began as a worktree manager: repos as bare clones, a read-only checkout of each default branch (the *ground*), a worktree per piece of work (a *capsule*). It also runs the work: a *mission* is a brief and the capsules it owns; agents (*crew*) work in them, an *orchestrator* agent decides what's next, and you review, approve and land from a web UI (`ws web`).

`MANUAL.md` is what ws does, for its users, and is kept current with the code.

## Build and test

```sh
just build     # the web UI into internal/web/dist, then bin/workspace. Always this, never go build -o
just check     # gofmt, go vet, go build ./..., and web/src/wire.gen.ts current
just wire      # web/src/wire.gen.ts again, from internal/web's types
just test      # go test ./..., then the web UI's tests (vitest)
just try up [seed...]       # try this build by hand: a fresh workspace in /tmp, seeded, its daemon and web UI up; prints dir, link, port, token and its ws
just try restart            # rebuild, and restart its daemon (same link)
just try down               # stop its daemon and remove it (it's also torn down when your session ends)
just test-workspace [dir]   # the same workspace, to keep: no daemon started, nothing torn down
just stories                # Storybook at localhost:6006: parts of the web UI drawn alone, every state side by side
just story-fixtures         # take the stories' missions again, from tries seeded for it
```

`ws eval` puts a role's prompt to fixtures, cases whose right answer is known, on a harness and model you name, and scores what comes back: for telling whether a change to a prompt, or another model in a role, does as well. It's off unless `WS_EVAL` is set, not listed and refused, and no agent ws starts is given the variable: it starts agents of its own, so it isn't for one to come on. It knows two roles (each one's `--help` says what a fixture is):

```sh
WS_EVAL=1 ws eval judge --pick claude:haiku internal/ops/testdata/judge/*    # the judge, on a step's work as its crew left it
WS_EVAL=1 ws eval orchestrator --pick claude:haiku \
  --before 'TRY_ROOT=$WS_EVAL_TMP scripts/try.sh up --no-build' \
  --after 'TRY_ROOT=$WS_EVAL_TMP scripts/try.sh down "$WS_EVAL_NAME"' \
  internal/ops/testdata/orchestrator/*    # the orchestrator, run for real on a mission a seed makes
```

The orchestrator's run is in a workspace made for it and removed after, which ws doesn't make: `--before` and `--after` are yours, and `scripts/try.sh` is what makes one here (`just build` first: `--no-build` uses `bin/workspace` as it is). A fixture's seed is a script as a try's is (`scripts/seeds`).

`internal/web/dist` is built, not committed: it's gitignored but for its `.gitkeep`, which `go:embed` needs. A binary built without it (`go build` alone, or `go install`) has no web UI, and `ws web` says so.

## Releases

Pushing a `v*` tag (`v1.4.0`; one with a `-`, `v1.4.0-rc.1`, is a prerelease) runs `.github/workflows/release.yml`: it builds ws with its web UI for darwin/arm64, linux/amd64 and linux/arm64, cgo on for code navigation, and publishes them as a GitHub release with checksums. A pull request that changes the workflow runs the builds without releasing. `ws upgrade` installs a release over the ws that's running (`internal/release`, which reads the same assets and `checksums.txt` as `scripts/install.sh`: change what a release publishes, and change both); a daemon restarts onto it once nothing's at work (`internal/daemon/restart.go`). The daemon looks once a day for ws's latest release, and for each plugin's that was installed from a repo (`ops.CheckUpdates`, from `internal/cli/updates_watch.go`): the web UI says what's out, and Settings' About upgrades (`ops.UpgradeWS`, the same `release.Upgrader` as `ws upgrade`).

## Where things are

```
cmd/workspace/        main: cli.NewRootCmd
internal/
  config/             ws.toml, ws.local.toml, ws.repo.toml: parsing and merging
  workspace/          the v1 core: repos, worktrees, git, status, silos, doctor, landing
  ide/ ui/            editor workspace files, styles and prompts

  mission/            the mission model and its store: state, stages, the court, decisions, the log
  harness/            the agent CLIs (cursor, cursor-acp, claude, codex, opencode, pi, shell): commands, hooks, transcripts
  crew/               starting and ending an agent on a mission
  orchestrator/       the agent that decides what's next: its prompt, its runs
  daemon/             the workspace's daemon: holds crew, runs jobs, hosts the web UI, watches PRs
  termpane/           a PTY and a terminal emulator
  conversation/       threads, talk, cells and passes: one event log per conversation
  page/               pages: markdown with cells an agent fills in
  artifact/           artifacts: versioned HTML prototypes, each a folder (artifact.toml, v1.html…) and a row in the records
  review/             your review of a capsule's diff
  github/ pulls/ team/  GitHub behind one client; pull requests outside missions; the Team page
  find/               the web UI's search index
  store/ words/       the workspace's records (.ws/ws.db, the daemon's): transactions, layouts, things, links and words; ranking what's said
  execlog/            .ws/exec.log: every process ws starts
  release/            ws's own releases: finding the latest, fetching one, putting it in place of the ws that's running (ws upgrade)

  ops/                the application layer: each operation that spans stores, once, and the views every front end shows
  doc/                a document's version, its kept versions and drafts, by scope (a mission's, or the workspace's)
  job/                work for an agent the server runs, one at a time per mission or conversation
  turn/               one headless turn of an agent: a command, or an ACP agent held for the turn
  prompt/             what agents are told (crew, conversations), beside the parsers of what they ask for, pinned by golden files
  git/ slug/          running git; making slugs
  web/                the HTTP server and its API (the app is embedded from web/dist)
  cli/                everything else (see below)
web/                  the web UI: React, TypeScript, Vite, ProseMirror
testdata/plugins/     frame-fixture: a plugin with a small app, for trying the page ws serves for a plugin's app (just try up frame-fixture)
```

## How it's layered, and where it isn't

- **The domain packages are leaves.** `mission` imports nothing of ws but `store` (and `git`, `slug`); `conversation` and `review` import `mission`, and `page` imports `conversation` (a cell is one); `harness` imports nothing of ws's but `acp`. A rule about one store is in that store's package (where a thread on a document is anchored, when a cell's stale, which passes a page has), not in `web` or `ops`. Nothing below `cli` prints: packages return data and `cli` formats it.
- **What a harness can do is data:** `harness.CapsOf(name)` (how its crew are held, where their transcript is, which roles it can play). Ask that, not a type assertion or its name.
- **`internal/ops` is the application layer.** An operation that spans two domain packages is written there once, takes who's asking (`ops.Actor`) and checks what they may do, and returns what happened as data with a sentence (`String()`), or one of its errors (`ErrNotFound`, `*Invalid`, `*Refused` with a code, `*Upstream`), which `web.writeError` maps to a status. Missions and stages are there (accept, answer, dismiss, state, brief, title, attach, detach), capsules (lift, dock, burn, land: in process, never by running `ws` again), crew's launch, adding, updating and removing a plugin, every write the web UI makes to threads, talk, pages, cells, passes, reviews and drafts, saving a document, the views the web UI reads, and answering a conversation (`AnswerConversation`: one loop, with a responder per `Conversation.What` saying which agent answers, what it's told and how its reply is read). A command loads one for its run (`loadOps`, or `LoadOps(dir)`), and works through it: there's no other handle on the workspace; the daemon opens one for its life (`ops.Open`), whose config is read again when `ws.toml` or `ws.local.toml` changes. `internal/cli` holds the cobra commands, which format what `ops` returns, and composes the daemon and the web server (`web*.go`). Add a new verb to `ops`, and call it from the command and the web UI. `internal/arch` tests which package may import which.
- **`web` asks one interface.** `web.Server.App` is a `web.App`, which `*ops.Workspace` is: every write a page makes, and every view it reads (`ops.Overview`, `Mission`, `Artifact`…), comes from it, as `ops`'s own types that `web` maps to its wire types (`web.WorkspaceOf`, `MissionOf`, `ArtifactContentOf`…). A handler that only reads a store (`conversation`, `mission`, `page`) reads it directly; none runs git or writes a store. `web` doesn't import `cli` or `workspace`, and `cli` builds none of `web`'s wire types: `newWeb` only composes the server. What `cli` gives `ops` (`ops.Options`) is where crew run (the host), the starters of the daemon's jobs, the clock, the PR cache and the Team service. Web's tests run on a real `ops.Workspace` (`testOps`), wrapped in a `fakeApp` where a test says otherwise.
- **The web UI's types of the API are generated from `web`'s.** `just wire` writes `web/src/wire.gen.ts` from `internal/web`'s exported types, with the types of other packages they use, named with the package in front (`review.File`: `ReviewFile`), by [guts](https://github.com/coder/guts) (`scripts/wiretypes`). Change a wire type in Go, run `just wire`, and `tsc` says what in the web UI no longer fits; CI fails if the file isn't current. `web/src/api.ts` re-exports the generated types (or names one as its own, `MissionTask = Task`); the rest it still declares by hand, mostly because the Go has `string` where the web UI has a union of strings. A named string type with its constants comes out as a union: the store's own (`mission.State`), or, for a field the store keeps as a plain string, one in `internal/web/enums.go` whose constants are the store's names for its values. Move a type by making the Go say what `api.ts` does, then dropping it from `api.ts`. An exported name in `web` that isn't sent or taken goes in `notWire`.
- **A mission's state is worked out, not stored.** `mission.DeriveState` and `mission.Resolve` give the state and whose court it's in to every front end. Don't re-derive either in a view.
- **How a headline is drawn is worked out there too.** `mission.Cards` makes each headline a card: its tone, a title, a sentence, the one action meant to move it (first, the only one accented) and what comes after, the quiet ones last, with the step the plan waits at folded into a question's card. The page draws a card as it's given (`CardView` and `CardActs` in `web/src/mission/Mission.tsx`), and knows only how each act is done. A new kind of headline, or a change to what one offers, is a change to `internal/mission/card.go`; the court's table of tests checks every case's cards against the rule, and they're the stories' *Every kind* (`just stories`).
- **What ws records is in `.ws/ws.db`,** which only the daemon opens: each store speaks SQL for its own tables, a write is one transaction (`store.Write`), a command's statements cross `store.sock`, and a write names what it changed. What's written to be read stays a file (briefs, handoffs, pages' text, versions, transcripts), written before its row. `ws doctor` sweeps files no row has; `ws export` writes a mission out as files. A write that matters has its error checked.

## Conventions

- **Words.** Ground, capsule, lift, dock, burn, land, board, silo: use these, not worktree, branch, delete. `MANUAL.md`'s Concepts section defines them.
- **Comments say why,** in plain sentences.
- **Commits are semantic** ([Conventional Commits](https://www.conventionalcommits.org)): `type(scope): subject`, and a pull request's title the same, since it's what a squash merge keeps. The type is one of `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `build`, `ci` or `chore`. The scope is optional: the area changed, a package or the web UI's part (`land`, `daemon`, `web`, `review`). The subject says why, in a plain sentence, lower case and without a full stop (`fix(land): a landing the mission can't record keeps the capsule, and says so`). A change that breaks a config, a command or a stored record adds `!` after the scope and says what breaks in a `BREAKING CHANGE:` footer.
- **Tests are behavioural.** `internal/integration` has tests that run real commands against a throwaway workspace (`testutil.SetupWorkspace`). Prefer one of those to a unit test of a helper. They're a package for each part of ws (`capsule`, `mission`, `orchestrate`, `artifact`, `doc`, `review`, `workspace`), so `go test` runs them side by side: a package's tests run one at a time, as they share the process's environment and output. Each package's `TestMain` is `testutil.Main`, what they share is in `internal/testutil`, and what they stand in for of `cli`'s own is in `internal/cli/seams.go`.
- **Agents are told, not trusted.** A rule that matters (crew don't land, crew don't accept their own stage) is enforced by the verb, as well as said in the prompt.
- **The web UI loads with `useLoaded`** (`web/src/load.ts`): its value and error, `set` for a write that answers with the value, `reload` for one that doesn't, and `soon` for what a load has so far. A load that sets several things begins with `useLatest()` and checks that it's still the latest before it sets anything: never set state from a `.then` unguarded. A hover card is `HoverCard` around a trigger, or `FloatingCard` with `useHoverDelay` for words the editor draws (`web/src/ui/hover.tsx`).
- **A story draws a part of the web UI from what ws gave,** never from a mission written by hand: `web/src/mission/fixtures` holds missions as the API answered for them, taken from seeded tries by `scripts/story-fixtures.mjs` (`just story-fixtures`), and a story (`*.stories.tsx`, beside the part) draws the part the page draws them with. A state worth looking at that no seed leaves a mission in is set up in that script with ws's own commands, or is a new seed. The stories' API answers only from what was taken, and refuses a write.
- **A new config key** goes in `internal/config` and `MANUAL.md` together. A key ws doesn't read is reported by `ws doctor`.
- **This checkout is shared.** More than one session may be working in it. Stage files by name, and don't fix or revert files you didn't change.
