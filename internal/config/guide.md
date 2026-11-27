# The guide to setting ws up

ws does a lot more once it's configured, and most of that is about the repos it holds. This guide is for an agent helping the user configure it. It covers what each setting is for, the signs in a repo that it's worth setting, and every key ws reads.

## How to help

1. Start from where the workspace stands, which is printed above this guide. It lists what each file already sets, which repos have no ws.repo.toml, and the signs found in each repo's ground.
2. Look at the repos before you recommend anything. Each one's default branch is at `repos/<repo>/.ground`. Read its package.json scripts, justfile, Makefile, compose file, README and CI workflows. Don't guess a command: take it from the repo.
3. Recommend the most useful settings first, and only what the workspace doesn't already set. Runs and capsule setup for the repos the user works in most usually come first.
4. For each recommendation, say what it gets the user in a sentence, which file it goes in, and give TOML that's ready to paste. When a setting could go in more than one file, choose by whose fact it is (see *The three files*).
5. Ask when you'd otherwise be guessing, such as which repo holds their documents, or whether a repo takes pull requests.
6. After a change, `ws doctor` lists the keys ws doesn't read, such as one that's misspelt or in the wrong file.

You don't change these files yourself. For a key the Settings page has a row for (listed above), point the user there. ws.toml is the team's, so the user commits it in the workspace's config repo. A ws.repo.toml is part of its repo's code and lands the way the repo's changes land: through a mission, or a capsule and a pull request.

## The three files

- **ws.toml** is the team's file, at the workspace's root, committed to the config repo. Put the team's choices here: the repos and groups, the agents everyone runs on by default, kinds of document, passes, plugins, the workspace's own runs.
- **ws.local.toml** is the user's file, beside ws.toml and not committed. The Settings page writes it, and it wins over ws.toml key by key. Put the user's own machine and taste here: a harness or model they prefer, notifications, the router, a repo only they work in.
- **ws.repo.toml** belongs to a repo, at its root, committed with its code and read from its ground. Put facts about the repo here, true in every workspace that has it: how a capsule is made workable, how it's run and tested, how work lands in it. Where ws.toml or ws.local.toml says the same thing for a repo, they win.

## What's worth setting up

### Runs: `[run.service.*]` and `[run.task.*]`

Recipes let `ws run <name>` start the repo's dev server, stack or tests on a free port, and end them when the work they're for is done. Crew are told each recipe by name, so they use `ws run` instead of starting servers by hand that clash over ports. A mission's checks and Try it steps can name recipes too. A service with a `page` becomes an app in the web UI.

Signs: package.json `scripts` (dev, start, storybook, test, lint, typecheck, build), justfile or Makefile targets, docker-compose.yml or compose.yaml, a Procfile, `.storybook/`, a README that says how to start it.

A service is something kept up until it's ended, like a dev server. A task ends by itself, like tests, lint or a build. A check is always a task. A task that needs a stack up names the service under `needs`.

```toml
# ws.repo.toml
[run.service.web]
command = "pnpm dev --port {port} --strictPort"
ready = "/"
description = "The storefront, on its dev server"

[run.task.test]
command = "pnpm test"
description = "Unit tests"
inputs = ["src/**", "package.json", "pnpm-lock.yaml"]   # cached: same files, same pass
```

The command must take its port from `{port}` or `$PORT`. A dev server that picks its own port, or moves to another when its port is taken, needs a flag that stops it (`--strictPort` for Vite). Recipes in ws.toml are the workspace's own, run at its root, for what belongs to no one repo.

### Capsule setup: `[capsule]`

A capsule is a fresh worktree, so anything gitignored isn't in it. `copy_from_ground` copies files such as `.env` from the ground. `after_create` runs the install or codegen. Without these, crew start in a capsule that won't build.

Signs: `.env.example`, `.env.sample` or a gitignored `.env` in the ground; a lockfile (pnpm-lock.yaml, package-lock.json, yarn.lock, uv.lock, poetry.lock, Gemfile.lock) whose install step a new worktree needs; a codegen step in the README.

```toml
# ws.repo.toml
[capsule]
copy_from_ground = [".env", ".env.local"]
after_create = "pnpm install --frozen-lockfile"
```

### Agents' guide, skills, process and PR writing: `[agents]`

Every agent ws starts is told where the workspace's guide and skills are. The planner and the orchestrator read the process, and the PR writer reads the PR guide. These are files at the workspace's root (AGENTS.md or CLAUDE.md, `.agents/skills`, PROCESS.md, PR.md), and `[agents]` is needed only to put them somewhere else. A workspace without them works the way ws does by default. Writing PROCESS.md and PR.md is often worth more than any key.

Signs: none of those files at the root. A repo's CONTRIBUTING.md or PR template that says how the team works.

### How work lands: `landing`, `verify`, `merge`

`landing = "direct"` is for a repo whose work goes straight to its default branch with `ws land`, with no pull requests: a notes or records repo. `verify` is the command that must pass before `ws land` pushes. `merge` is how ws merges the repo's pull requests. A repo says the first two of itself in ws.repo.toml. ws.toml says them, and `merge`, under `[repos.<name>]`.

Signs: a repo nobody opens pull requests on, or with no branch protection; a GitHub merge setting other than squash.

### Records and documents: `[missions] records`, `documents`, `[documents.*]`

`[missions] records` names the repo where the workspace's plans, decisions and notes go. A repo that's mostly documents can be a space of documents (`documents = true`, or a folder of it with `documents = "docs/"`). It's then read in the Documents page with threads and passes instead of in Files. `[documents.<kind>]` gives a kind of document (an ADR, an RFC) its place, its name and its commits, so agents propose them the way the team writes them.

Signs: a repo or folder of markdown (docs, adr, rfcs, notes, records, handbook); numbered or dated file names; a template file.

```toml
# ws.toml
[missions]
records = "records"

[documents.adr]
title = "Decision record"
path = "records:adr/{number}-{slug}.md"
add = "docs: add ADR {number} {slug}"
```

### Repos, names and groups: `[repos.*]` and `[groups.*]`

Aliases (`fe` for frontend) make commands shorter. A display name and colour make lists easier to read. A group gathers related repos in the web UI, and a hidden group stays out of commands that cover every repo. A repo of another org, or one whose name is taken, says where it is with `github = "owner/name"`.

Signs: more than a handful of repos; shared prefixes or suffixes (svc-, lib-, -infra); repos the user never works in.

### Agents for each role: `[crew]`, `[orchestrator]`, `[conversation]` and the rest

These set which harness (claude, cursor-acp, cursor, codex, opencode, pi) and model runs each role: crew, the planner (`[briefing]`), the orchestrator, conversations, the judge, the PR writer, the Team page's writer, automations. Leave them unset to use ws's defaults. Most have a row in Settings › Agents. `[crew.levels]` puts rote steps on a small model, `[crew] allowed` lists what else crew may be started on, and `[orchestrator] dispatch = "ask"` holds crew until the user starts them.

Signs: which harnesses are installed, what the team pays for, and whether spend matters.

### Guarantees: `[missions]`

These are rules ws holds for every mission, whatever an agent reads. `review_before_pr` holds a pull request until the user's review approves it, `review_before_land` does the same for a direct repo, and `review_before_ready` keeps a draft until then. `max_rounds` is how many times checks may come round unmet before the step goes to the user. Recommend them to a team that wants a person on every change.

### Reviews and pages: `[reviews]` and `[pages]`

A pass is a review by an agent, of a pull request (`[reviews.passes.<name>]`) or a page (`[pages.passes.<name>]`), from a checklist file, a skill, or a line of what to look for. `auto = true` runs it without being asked on a pull request that needs the user, and `blocks` makes its findings hold a merge. `[reviews.guide]` says what the walk-through of a change should attend to.

Signs: a security, accessibility or API checklist in a repo; a skill that reviews; review rules in CONTRIBUTING.

### The Team page: `[team]`

`[team.people.<key>]` joins someone's logins and emails into one person with a name. `[team.watch.<name>]` marks changes to paths that change how things are done, such as CI, lint config or guides. `window` is how far back the page looks.

Signs: the same person under several emails in the git log; a bot account; config folders the team cares about.

### Plugins: `[plugins]` and `[plugin.*]`

`[plugins.<name>]` says where a plugin the team uses comes from, and `ws doctor` says which aren't installed. `[plugin.<name>]` is that plugin's own config, and what its keys are is the plugin's to say: `ws show plugins <name>` has its guide. `[things.<kind>]` says where a plugin's kind of thing goes when it's put in a repo.

### Look and feel, and the user's machine

`[ui]` sets the accent colour, the tint and the sidebar's order. Mostly that's the user's, through Settings. In ws.local.toml alone: `[router]` gives the workspace and its runs names under .localhost, `[notifications]` says what's told as it happens, `[updates]` controls looking for new releases, and `git = "ssh"` clones over SSH.

## Every key

Keys under a table named by the file (`[repos.frontend]`) are written here with `<name>`. A table listed for ws.toml and ws.local.toml can be set in either, and ws.local.toml wins.

### [workspace] (ws.toml)

- `workspace.org`: the GitHub owner a repo is under when it doesn't say its own `github`.
- `workspace.default_branch`: the branch every ground is on (main).
- `workspace.display_name`: the workspace's name in prompts and the web UI.

### [repos.\<name>] (ws.toml, ws.local.toml)

A repo only in ws.local.toml is the user's own and isn't in a teammate's workspace. Aliases are added to the team's list. Every other key replaces the team's value.

- `repos.<name>.display_name`: what it's called where it's shown.
- `repos.<name>.aliases`: short names for it in commands.
- `repos.<name>.color`: its colour, hex or a 256-colour code.
- `repos.<name>.after_create`: a command run in a new capsule. It wins over ws.repo.toml's.
- `repos.<name>.group`: the group it's in.
- `repos.<name>.landing`: `"pr"` (the default) or `"direct"`.
- `repos.<name>.verify`: a command that must pass before `ws land` pushes.
- `repos.<name>.merge`: `"squash"`, `"merge"` or `"rebase"`. Unset, ws squashes where GitHub allows it.
- `repos.<name>.documents`: `true`, a folder (`"docs/"`), or `false`, for whether it's a space of documents.
- `repos.<name>.github`: `owner/name` on GitHub, when that isn't the org's repo of its name.

### [groups.\<name>] (ws.toml, ws.local.toml)

- `groups.<name>.hidden`: leave its repos out of commands that cover every repo.
- `groups.<name>.display_name`: what it's called in the web UI.
- `groups.<name>.order`: where it comes among the groups, lowest first.

### [capsule] (ws.repo.toml)

- `capsule.copy_from_ground`: files copied from the ground into each new capsule. Missing ones are skipped.
- `capsule.after_create`: a command run in each new capsule, unless the workspace sets one for the repo.

### Landing (ws.repo.toml, at the top)

- `landing`: the repo's own `landing`.
- `verify`: the repo's own `verify`.
- `documents`: the repo's own word on whether it's a space of documents.

### [silo] (ws.repo.toml; ws.local.toml)

- `silo.after_switch`: a command run in the repo's silo after it's pointed at another capsule, such as restarting a service that doesn't reload (ws.repo.toml).
- `silo.after_change`: a command run in the silo after each sync that changed something, such as codegen (ws.repo.toml).
- `silo`: which capsule each repo's silo points at (ws.local.toml). ws writes this itself with `ws silo point`.

### [run.service.\<name>] (ws.repo.toml; ws.toml for the workspace's own)

- `run.service.<name>.command`: what's run, in a shell. `{port}` and `$PORT` are its port.
- `run.service.<name>.display_name`: what it's called in the web UI.
- `run.service.<name>.description`: what it is and when it's wanted, in a line. Agents are told it.
- `run.service.<name>.kind`: `"app"` (what's being built, the default) or `"tool"` (Storybook, API docs, a database UI).
- `run.service.<name>.render`: `"inline"` (framed, the default) or `"tab"` (a browser tab of its own).
- `run.service.<name>.ready`: a path that answers below 500 once it's up. Unset, it's up once its port listens.
- `run.service.<name>.stop`: a command run after it's ended (`docker compose down`).
- `run.service.<name>.where`: `""` for the worktree you stand in, or `"silo"` for the repo's silo.
- `run.service.<name>.page.title`: makes it an app in the web UI, under this title.
- `run.service.<name>.page.icon`: the app's icon, one of the web UI's by name (`book`).
- `run.service.<name>.needs.<name>.run`: a recipe it needs up first, `"server"` or `"backend/server"`. Unset, the recipe with the need's own name.
- `run.service.<name>.needs.<name>.repo`: a repo whose folder it needs, in place of a run.
- `run.service.<name>.needs.<name>.env`: the variable the need's address is handed over in (`API_URL`).
- `run.service.<name>.needs.<name>.mount`: a path of this run's own name the need answers at (`/api`), through the router.
- `run.service.<name>.needs.<name>.strip`: take the mount's path off before the need is asked.

### [run.task.\<name>] (ws.repo.toml; ws.toml for the workspace's own)

- `run.task.<name>.command`: what's run, in a shell, to its end.
- `run.task.<name>.display_name`: what it's called in the web UI.
- `run.task.<name>.description`: what it is and when it's wanted, in a line. Agents are told it.
- `run.task.<name>.stop`: a command run after it's ended or been stopped.
- `run.task.<name>.where`: as a service's.
- `run.task.<name>.timeout`: how long it may take (`10m`). Unset, an hour.
- `run.task.<name>.inputs`: the files it reads, which makes it cached: the same files, command and env reuse its last pass.
- `run.task.<name>.outputs`: the files it writes that a reuse puts back.
- `run.task.<name>.env`: the variables it reads, which count as inputs.
- `run.task.<name>.after`: tasks of the same worktree run, or reused, before it.
- `run.task.<name>.reports.<name>.path`: a file its command writes that's kept with its result (`cover.out`, `junit/*.xml`).
- `run.task.<name>.reports.<name>.format`: what that file is written in (`lcov`, `gocover`, `junit`), for a plugin to show.
- `run.task.<name>.needs.<name>.run`: as a service's, and ended once the task has.
- `run.task.<name>.needs.<name>.repo`: as a service's.
- `run.task.<name>.needs.<name>.env`: as a service's.
- `run.task.<name>.needs.<name>.mount`: as a service's.
- `run.task.<name>.needs.<name>.strip`: as a service's.

### [agents] (ws.toml, ws.local.toml)

- `agents.guide`: the workspace's guide for agents. Unset, AGENTS.md or CLAUDE.md at the root.
- `agents.skills`: the folder of its skills, each a folder with a SKILL.md. Unset, `.agents/skills` or `.claude/skills`.
- `agents.process`: how work goes here, in prose. Unset, PROCESS.md at the root.
- `agents.pr`: how pull requests are written here, for the PR writer. Unset, PR.md at the root.

### Roles (ws.toml, ws.local.toml)

`harness` is the agent CLI, and `model` is passed to it in that harness's own naming (`opus` for claude). `in` is read for `[briefing]` alone.

- `crew.harness`: what crew run on.
- `crew.model`: crew's model.
- `crew.in`: not read for crew.
- `crew.allowed`: other `harness:model` picks crew may be started on from a mission's page.
- `crew.levels.rote`: what a rote step runs on. Unset, the harness's small model.
- `crew.levels.normal`: what a normal step runs on. Unset, crew's own.
- `crew.levels.hard`: what a hard step runs on. Unset, crew's own.
- `crew.trust_folders`: answer Claude Code's trust dialog yes for crew, for a throwaway workspace. Read from ws.local.toml alone.
- `briefing.harness`: what the planner runs on.
- `briefing.model`: the planner's model.
- `briefing.in`: `"web"` (a conversation, the default) or `"terminal"`.
- `orchestrator.harness`: what the orchestrator runs on. Unset, crew's harness.
- `orchestrator.model`: the orchestrator's model.
- `orchestrator.enabled`: whether it runs by itself when crew report (on unless set off).
- `orchestrator.dispatch`: `"auto"` (the default) starts crew at once, and `"ask"` holds them for the user.
- `orchestrator.max_dispatches`: how many crew it may start in a row before it has to ask (3).
- `conversation.harness`: what conversations, threads, cells and passes run on.
- `conversation.model`: their model.
- `conversation.in`: not read for conversations.
- `conversation.turn_minutes`: how long a turn that runs things may take (60, at most 240).
- `judge.harness`: what judges a step's checks in words. Unset, the planner's.
- `judge.model`: its model. Unset, the planner's small model.
- `judge.in`: not read for the judge.
- `pr_writer.harness`: what writes the pull requests ws opens. Unset, conversations'.
- `pr_writer.model`: its model.
- `pr_writer.enabled`: `false` titles a pull request as its mission is, with no agent.
- `automations.harness`: what answers an automation's ask.
- `automations.model`: its model.
- `automations.ask_budget`: tokens a day each automation's asks may spend (0 is ws's default, below 0 no limit).

### [missions] (ws.toml, ws.local.toml)

- `missions.records`: the repo plans, decisions and notes go in.
- `missions.review_before_pr`: open a mission's pull request only once the user's review approves it.
- `missions.review_before_land`: land a direct repo's capsule only once the user's review approves it.
- `missions.review_before_ready`: keep a mission's draft pull request a draft until the user's review approves it.
- `missions.replies_need_me`: hold a reply drafted to a person on a pull request until the user approves it (on unless set off).
- `missions.end_when_merged`: end a mission by itself once its work has merged and nothing waits (on unless set off).
- `missions.max_rounds`: how many times a step's checks may come round unmet before it's the user's (3).

### [documents.\<name>] (ws.toml)

- `documents.<name>.title`: what one's called. Unset, the kind's name.
- `documents.<name>.path`: where one goes, as `repo:path`, with `{date}`, `{slug}` or `{number}`, ending `.md`.
- `documents.<name>.add`: the first line of the commit that adds one.
- `documents.<name>.amend`: the first line of the commit that changes one.
- `documents.<name>.guide`: the file that says how one's written.
- `documents.<name>.passes`: the workspace's page passes that are this kind's.
- `documents.<name>.moves.<name>.to`: a folder one moves to at a step in its life (accepted/).
- `documents.<name>.moves.<name>.commit`: the first line of that move's commit.
- `documents.<name>.moves.<name>.asks`: what that commit's body asks.

### [artifacts] (ws.toml; path in ws.local.toml too)

- `artifacts.path`: where a promoted artifact lives, as `repo:folder/{date}-{slug}/` in a direct repo. Unset or `".ws"`, on this machine.
- `artifacts.promote`: the first line of the commit that promotes one.
- `artifacts.discard`: the first line of the commit that discards one.
- `artifacts.restore`: the first line of the commit that restores one.
- `artifacts.expire_after`: how long a live one lasts after its last version (`"30d"`, or `"never"`).
- `artifacts.guide`: a file that says how one's written, in place of ws's own rules.

### [things.\<name>] (ws.toml)

- `things.<name>.paths`: the places a plugin's kind of thing may go, as `repo:folder/{slug}/`.
- `things.<name>.subfolders`: let one be grouped in a folder of its place.

### [pages] (ws.toml)

- `pages.style`: a file saying how what agents write into pages should read.
- `pages.passes.<name>.title`: what the pass is called.
- `pages.passes.<name>.about`: what it's for, in a line.
- `pages.passes.<name>.looks`: what its agent looks for.
- `pages.passes.<name>.file`: a file that says how to do it, such as a checklist.
- `pages.passes.<name>.skill`: a skill of the workspace's its agent follows.
- `pages.passes.<name>.args`: what's asked of that skill.
- `pages.passes.<name>.kinds`: the kinds its findings can be.
- `pages.passes.<name>.cold`: give its agent the page and nothing else.

### [reviews] (ws.toml, ws.local.toml)

A pass in ws.local.toml is added to the team's, and replaces one of the same name.

- `reviews.guide.looks`: what the walk-through of a change should attend to.
- `reviews.guide.file`: a file that says.
- `reviews.guide.auto`: write it without being asked, for a pull request that needs the user.
- `reviews.passes.<name>.title`: what the pass is called.
- `reviews.passes.<name>.about`: what it's for, in a line.
- `reviews.passes.<name>.looks`: what its agent looks for.
- `reviews.passes.<name>.file`: a file that says how, such as a checklist.
- `reviews.passes.<name>.skill`: a skill of the workspace's its agent follows.
- `reviews.passes.<name>.args`: what's asked of that skill.
- `reviews.passes.<name>.kinds`: the kinds its findings can be.
- `reviews.passes.<name>.blocks`: the kinds whose open findings hold the change back.
- `reviews.passes.<name>.cold`: give its agent the diff and description alone.
- `reviews.passes.<name>.auto`: run it without being asked on a pull request that needs the user.
- `reviews.passes.<name>.repos`: the repos it's for. Unset, all of them.

### [team] (ws.toml, ws.local.toml)

- `team.window`: how far back the Team page first looks (`14d`, `2w`).
- `team.narrative`: whether an agent writes a gist of each change (on unless set off).
- `team.harness`: what writes those gists. Unset, conversations' on its small model.
- `team.model`: its model.
- `team.people.<name>.name`: someone's name.
- `team.people.<name>.logins`: their GitHub logins.
- `team.people.<name>.emails`: their commit emails.
- `team.watch.<name>.title`: what a change to these paths is marked with.
- `team.watch.<name>.paths`: the paths, as globs, with `repo:` before one for one repo's.

### [plugins] and [plugin] (ws.toml; plugin in ws.local.toml too)

- `plugins.<name>.source`: where a plugin the team uses comes from.
- `plugins.<name>.version`: the version to install.
- `plugin`: each plugin's own config, under `[plugin.<name>]`. Its keys are the plugin's: `ws show plugins <name>`.

### [ui] and [web] (ws.toml, ws.local.toml)

- `ui.accent`: the web UI's accent colour, `#rgb` or `#rrggbb`.
- `ui.tint`: how far its greys lean towards the accent, 0 to 100.
- `ui.sidebar`: the sidebar's places in order, by id. Settings › Arrange writes it.
- `ui.menu`: the workspace menu's places in order, by id.
- `web.port`: the port the web UI listens on. 0 picks one from the workspace's path.

### The user's machine (ws.local.toml)

- `git`: `"ssh"` or `"https"` (the default), for cloning.
- `boarded`: which capsules are boarded in the IDE. ws writes this itself.
- `router.enabled`: give the workspace and its runs names under .localhost.
- `router.name`: the workspace's name there. Unset, its folder's.
- `updates.check`: look once a day for later releases of ws and plugins (on unless set off).
- `notifications.missions`: `"feed"`, `"desktop"` or `"off"`, for a mission ending or started for the user.
- `notifications.pulls`: for a pull request this ws opened merging.
- `notifications.plugins`: for a plugin's update.
- `notifications.ws`: for ws being upgraded.
- `notifications.automations`: for what automations say.
- `notifications.plugin`: each plugin's kinds of notification, under `[notifications.plugin.<name>]`.
- `github.fixture`: a JSON file standing in for GitHub, for a test workspace.
