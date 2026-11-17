# Settings

Status: built on `v2`, 2026-11-11: all four slices, and the four questions decided (below). See [What's built](#whats-built). It came from reading how ws reads and writes its config; the user decided the open questions.

A page in the web UI where you change your own settings, the ones in `ws.local.toml`: which agent runs each role and on which model, whether the orchestrator runs and how far it goes alone, the Team page's agent. It says what each setting is now and where that comes from (ws's default, the team's `ws.toml`, or yours), and writes your changes into `ws.local.toml` without disturbing the rest of the file.

## Why

Choosing an agent today means editing `ws.local.toml` by hand. That's how `[conversation]` went to Codex in climate-risk, and it shows the gaps:

- **You have to know the names.** Harnesses (`cursor`, `cursor-acp`, `claude`, `pi`, `codex`, `shell`), which roles each can run (Codex and `cursor-acp` only the headless ones), and models in each agent's own naming. Nothing tells you while you type.
- **Settings reach further than they look.** With no `[team]` section, the Team page's agent follows `[conversation]`. Moving conversations to Codex would have moved Team's gists too, on an API key, had I not pinned `[team]`.
- **You can't see what's in effect.** The Orchestrator page shows its agent and model, read-only, with "`[orchestrator]` in `ws.toml` or `ws.local.toml`". The other roles show nothing.
- **A mistake shows up later.** A misspelt harness fails the next run; a misspelt key is ignored without a word (`ws doctor` lists it).

## What ws does with config now

- **Reading.** `config.Load` reads `ws.toml`, then merges `ws.local.toml` over it, key by key. The web UI's handlers load it afresh on every request (`LoadContextFromDir`), and so does every command. The deck reads it once, when it opens (`m.cfg`), so a change doesn't reach an open deck.
- **Writing.** ws writes `ws.local.toml` itself when you lift (`[boarded]`), point a silo (`[silo]`) or hide a group (`[groups]`), all through `config.UpdateLocal`. That decodes the file, changes it and encodes it whole again. It keeps keys it doesn't read, but **comments and layout are lost**, and keys come back in its own order and indentation. It isn't locked, and it writes in place, not by rename, because the silo watcher watches the file.
- **Checking.** `Load` refuses a bad `dispatch`, `git` protocol, group name or `landing`. A harness name isn't checked until something runs it. Keys ws doesn't read are kept in `Config.Unknown` for `ws doctor`.

## What the page has

Your settings only, in `ws.local.toml`. The team's `ws.toml` is shown, not changed.

| Group | Settings (section, key) | Values |
|---|---|---|
| **Agents** | `[crew]` harness, model | a harness that runs crew; a model in its naming |
| | `[briefing]` harness, model, in | as crew's; `web` or `terminal` |
| | `[orchestrator]` harness, model | a harness that runs headless |
| | `[conversation]` harness, model | a harness that holds a conversation |
| | `[team]` harness, model | as conversation's |
| **Orchestrator** | `[orchestrator]` enabled, dispatch, max_dispatches | on/off; `auto` or `ask`; a number |
| **Team** | `[team]` narrative, window | on/off; `14d`, `2w`… |
| **Repos** | `git` | `ssh` or `https` |

Each setting shows:

- **Its value now, and where it comes from:** *default*, *team* (`ws.toml`) or *yours*.
- **For a role, what actually runs** (`harness.Roles`: the harness and model each role ends up with, with what it falls back on). It warns when the harness can't run here (`Available()`: "codex-acp not found…") or can't run that role.
- **What it reaches.** For example: "Team's agent follows this, as `[team]` sets none."
- **Your value can be cleared,** which removes it from `ws.local.toml` so the team's or the default applies again.

Models are free text with suggestions: the models the harness's own CLI lists, without a session: `cursor-agent --list-models`, `pi --list-models`, Codex's catalog (`codex debug models`, through `codex-acp cli`), each at each effort, and Claude Code's aliases, as it has no such command. (I'd first thought listing needed a session; the user pointed out that CLIs list their models.)

**Left off the page** (file only):
- **Commands:** `[repos.<r>] after_create` and `verify` run in a shell. A page that writes them would let anyone signed into the web UI run a command, and these are the team's anyway.
- **Paths:** `[agents] guide` and `skills`, `[github] fixture`. They're rare and easy to get wrong in a form.
- **What ws writes itself:** `[boarded]` and `[silo]` are state, not settings. Group visibility already has its own controls.
- **The team's:** documents, passes and pages are `ws.toml`'s.
- **Keys ws doesn't read** in `ws.local.toml` are listed, as `ws doctor` lists them, with the line they're on.

## How it writes

**A setting is a key, edited where it is.** `config.SetLocal(root, section, key, value)` and `config.UnsetLocal(root, section, key)` edit the file's text:
- they replace the key's line if it's there;
- otherwise they add it under the section's header, or add the section at the end;
- the comments, the order and the indentation stay as you left them.

Then:

1. **Validate the value** before writing: a harness that's registered and can play the role, `dispatch` one of two, a positive number, a window that parses.
2. **Read the result back** (`config.Load`). If it doesn't load, or doesn't say what was set, nothing is written and the page says why.
3. **Lock while writing** (`filelock`, as the stores are), so a lift writing `[boarded]` at the same moment can't lose the setting, or the setting the lift.

A file written in a shape the editor can't place a key in safely (dotted keys at the top, an inline table) is left alone, and the page says to edit it by hand.

## Where a change takes effect

| What | When |
|---|---|
| The web UI, every command, every run ws starts | At once: they read the config each time |
| An open deck | When it reopens (lean: when it notices the file changed; see below) |
| `[web] port` | Not on the page: the server's port is set when it starts |

## API and page

- `GET /api/settings`: each setting with its value, its source, what's allowed, and for roles what runs and any problem; plus the unknown keys.
- `POST /api/settings {section, key, value}` sets one; `{section, key, clear: true}` clears it. The answer is the settings as they now are, so the page shows what took effect.
- In `web`, hooks set in `cli/web.go` as usual (`web` doesn't import `config` or `workspace`).
- A **Settings** item at the foot of the sidebar. The Orchestrator page's "Settings" row links to it.
- Each change saves on its own, with a toast that says what changed and in which file ("`[conversation] harness = "codex"` in ws.local.toml"), and *Undo*.

## Slices

1. **Read only.** The page with every setting's value and source, what each role runs, and the unknown keys. That's useful on its own: it answers "what runs my conversations?"
2. **The writer.** `SetLocal`/`UnsetLocal`, comment-keeping, locked and checked, with tests on files written by hand (comments, odd spacing, missing sections) and by ws.
3. **Agents and the orchestrator, editable.**
4. **The rest** (Team, git), and the deck reloading its config.

## Decided

- **ws's own writes keep comments too: yes.** Lifts, silos and groups go through the same key-by-key writer.
- **The deck reloads: yes.** It checks the two files' modification times on its tick.
- **The team's `ws.toml` is read-only here: yes.** A change to it is a change for everyone, which belongs in a commit.
- **Models come from each harness's CLI,** which lists them without a session.
- **Per-mission overrides:** not now. Nothing reads one, and the deck's `L` already picks a harness for one launch.

## What's built

- **The writer** (`config.UpdateLocal`, `EditLocal`, in `internal/config/edit.go`). It compares what the file said with what it's to say, value by value, and edits only those lines:
  - a value is replaced where it is, keeping a comment after it;
  - a key is added after its table's last, indented like them;
  - a new table goes at the end;
  - an emptied table goes, leaving one blank line.

  Sets come before removals, so a table that loses a key as it gains one stays put. The result is read back and must say exactly what was meant. Writes are locked (`ws.local.toml.lock`). An empty string, a zero and an empty list aren't written: each means unset, and the old writer left `git = ""` and `max_dispatches = 0` behind. A file it can't edit key by key is refused by `EditLocal` (the page) and written whole by `UpdateLocal` (ws's own writes, which mustn't fail over layout).
- **`ws.local.toml` keeps your comments** through lifts, silos, groups and `ws init`.
- **The API** (`internal/cli/settings.go`, `internal/web/settings.go`):
  - `GET /api/settings` gives every setting with its value, its source, each file's own value and the default (worked out by resolving the role with the key unset), and for each role what it runs, its problem and what it follows.
  - `POST /api/settings` takes changes together (a harness and its model), checked on a copy first, all or nothing.
  - `GET /api/settings/models` asks the harness's CLI, and keeps the answer ten minutes.
- **The page** (`web/src/settings/Settings.tsx`, *Settings* in the sidebar; the Orchestrator page links to it):
  - Each group shows what runs, then its settings, each with a source chip (default, team, yours) and Reset.
  - Changing a role's agent clears your model for it, in the same write.
  - Each change toasts what was written and where, with Undo; a refusal toasts why.
- **The deck reloads its config** when either file changes (`deckModel.reloadConfig`, on its tick). A file that can't be read keeps the last config and says so.
- **Tried in Chrome** on a throwaway workspace:
  - changing conversations' agent to Codex cleared the model set for the old one, and Undo restored the edit;
  - Codex's 59 models and efforts were offered;
  - the orchestrator was switched off and on, and a model reset to its default;
  - `max_dispatches = 0` was refused with why;
  - the page fits a phone's width.

  The file kept its comments throughout. `max_dispatches = 0`, left by the old writer, shows as the default 3: zero means unset.
