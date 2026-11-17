build: web
    go build -trimpath -ldflags="-s -w" -o bin/workspace ./cmd/workspace

# Build the web UI (web/) into internal/web/dist (gitignored), which the binary embeds.
web:
    #!/usr/bin/env sh
    set -e
    if ! command -v pnpm >/dev/null; then echo "pnpm not found: building without the web UI"; exit 0; fi
    cd web
    [ -d node_modules ] || pnpm install --frozen-lockfile --silent
    pnpm --silent build --logLevel warn

# Work on the web UI with hot reload at localhost:5173, against a workspace's running web UI (ws web starts it).
web-dev ws=(env_var('HOME') + "/Projects/ws-test"):
    cd web && [ -d node_modules ] || pnpm install --frozen-lockfile
    cd web && WS_ROOT="{{ws}}" pnpm dev

check:
    gofmt -l . | grep . && exit 1 || true
    go vet ./...
    go build ./...

# Go's tests, then the web UI's (web/src/**/*.test.ts), if pnpm is here.
test:
    go test ./...
    @if command -v pnpm >/dev/null; then cd web && { [ -d node_modules ] || pnpm install --frozen-lockfile --silent; } && pnpm --silent test; else echo "pnpm not found: skipping the web UI's tests"; fi

cover:
    go test ./... -coverprofile=cover.out && go tool cover -func=cover.out

cover-html:
    go test ./... -coverprofile=cover.out && go tool cover -html=cover.out

# Build a throwaway workspace for trying missions by hand (Claude Code on haiku in every role).
test-workspace dir=(env_var('HOME') + "/Projects/ws-test"):
    ./scripts/test-workspace.sh "{{dir}}"
