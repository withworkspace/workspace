# ws

Workspace management CLI tool written in Go.

## Building

Always build for development using `just build`. This produces the binary at `bin/workspace`.

Do not use `go build` directly or produce binaries elsewhere.

## Testing

Run tests with `just test`.

To try missions, the web UI and agents by hand, `just test-workspace [dir]` builds a throwaway workspace (default `~/Projects/ws-test`): repos `demo` and `records` with local remotes (`records` has no pull requests: `landing = "direct"`, landed with `ws land`), Claude Code on haiku in every role, and a file standing in for GitHub (`.remotes/github.json`: open pull requests on `demo`, a stack among them, for the web UI's Reviews page). Remove the folder and run it again for a fresh one.
