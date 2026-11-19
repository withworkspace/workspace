# ws

Workspace management CLI tool written in Go.

## Building

Always build for development using `just build`. This produces the binary at `bin/workspace`.

Do not use `go build` directly or produce binaries elsewhere.

## Testing

Run tests with `just test`.

To try a change by hand, `just try up [seed...]` is the one command: it builds ws, makes a fresh throwaway workspace in `/tmp/ws-try/<name>`, runs the seeds you name (`just try seeds` lists them: each puts something particular there, such as a mission mid-check), starts its daemon and web UI on this checkout's build, and prints where it is (`dir`, the sign-in `link`, `origin`, `port`, the `token` for `Authorization: Bearer`, and `ws`). Run ws there only through the `ws` it prints (`<dir>/.try/ws`): it drops your mission, crew and agent session, which `bin/workspace` run from a capsule would act for, in the workspace this work runs from. After a change, `just try restart` rebuilds and restarts the daemon on the same link. `just try down` stops the daemon and removes the folder; a try is also torn down by itself when the session that made it ends, or after four hours. A new thing worth looking at more than once is a new seed, `scripts/seeds/<name>.sh` (`scripts/try.sh` says what one is given).

For a workspace to keep, `just test-workspace [dir]` builds a throwaway workspace (default `~/Projects/ws-test`): repos `demo` and `records` with local remotes (`records` has no pull requests: `landing = "direct"`, landed with `ws land`), Claude Code on haiku in every role, and a file standing in for GitHub (`.remotes/github.json`: open pull requests on `demo`, a stack among them, for the web UI's Reviews page). Remove the folder and run it again for a fresh one.

The README's and `FEATURES.md`'s pictures are of the `marketing` seed: Sorrel, a made-up company's workspace, with missions in every court and no model behind them (`scripts/seeds/marketing.sh` says what it's made of). `just shots` takes the README's again, and `just shots all` every view's, for `FEATURES.md`, into `docs/assets/shots/`: it makes a try with that seed, opens its pages in a headless Chrome, light and dark, and tears it down. A picture worth taking more than once is a line in `scripts/shots.mjs`; what it shows goes in the seed, so the pictures stay of one workspace.
