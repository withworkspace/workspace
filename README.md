<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/wordmark-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/assets/wordmark-light.png">
    <img alt="workspace" src="docs/assets/wordmark-light.png" width="420">
  </picture>
</p>

<p align="center">
  Multi-repo worktree first development for humans and agents.
</p>

---

**workspace** gives your multi-repo setup a single control plane. Git worktrees become lightweight capsules you can lift, dock, and debrief — while your IDE stays in sync automatically.

## Get started

Point `ws init` at a config repo containing your `ws.toml` and everything gets cloned and wired up:

```sh
ws init git@github.com:your-org/workspace-config.git
```

## How it works

Define your repos once in `ws.toml`:

```toml
[workspace]
org = "your-org"
default_branch = "main"

[repos.frontend]
display_name = "Frontend"
aliases = ["fe"]

[repos.backend]
display_name = "API"
after_create = "make setup"
```

Then use capsules (worktrees) to work on branches without ever switching:

```sh
ws lift frontend my-feature    # create a new capsule
ws dock frontend 1234          # check out a PR by number
ws jump fe                     # fuzzy-navigate between capsules
ws rg useAuth                  # ripgrep across repos, one worktree each
ws debrief                     # clean up anything that's landed
```

## Commands

| Command | What it does |
|---------|-------------|
| `ws` | Show status across all repos |
| `ws lift` | Create a new capsule from a base branch |
| `ws dock` | Check out an existing branch or PR |
| `ws jump` | Navigate to any capsule with fuzzy matching |
| `ws rg` | Ripgrep across repos, searching one worktree per repo |
| `ws debrief` | Remove landed and stale capsules |
| `ws group` | List repo groups and choose which are hidden |
| `ws mission` | Track a unit of work: its brief, capsules across repos, and log |
| `ws web` | Open the web UI: missions, crew, reviews, conversations, pages |
| `ws open` | Open workspace in Cursor, VS Code, or IntelliJ |
| `ws board` | Add a capsule to your IDE workspace |
| `ws doctor` | Health check your workspace |
| `ws upgrade` | Pull latest config and set up new repos |

## Install

From a release, with its web UI and code navigation, for macOS on Apple silicon and Linux (amd64, arm64). It downloads with [gh](https://cli.github.com), logged in:

```sh
gh api -H 'Accept: application/vnd.github.raw' repos/withworkspace/workspace/contents/scripts/install.sh | bash
```

That installs the latest release at `~/.local/bin/workspace`, checked against the release's checksums. From a clone, it's `scripts/install.sh [version] [dir]`: a tag (`v0.0.3`) for another release, and another directory (or `$WS_INSTALL_DIR`). Run it again to upgrade.

Or build it:

```sh
go install github.com/withworkspace/workspace/cmd/workspace@latest
```

That builds ws without its web UI (`ws web` says so). For the web UI, build from a clone with `just build` (Go, Node, [pnpm](https://pnpm.io) and [just](https://github.com/casey/just)) and put `bin/workspace` on your `PATH`.

Then add to your `~/.zshrc`:

```sh
eval "$(workspace shell-init zsh)"
```

This gives you the `ws` wrapper with shell completions and `cd` integration.

## License

MIT
