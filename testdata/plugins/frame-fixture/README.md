# frame-fixture

A plugin for trying ws's page for a plugin's app (the shell) and the frame protocol by hand. It isn't published, and it isn't built on the plugin framework: it speaks the plugin protocol's JSON-RPC by hand (`main.go`), so ws can be tried before the framework builds apps itself.

- **`main.go`** registers two views and its app (`ui.script`, `ui.style`), and serves the app's files at `/ui/` and one JSON endpoint, `/api/things`. It serves no HTML. It registers how its things are named too (`refs`: `THING-2` is the Lantern), answers `ref/show` for one, and tells ws of all three as it starts (`thing/put`), so Find has them: `just try up plugin-refs` seeds a mission and a note that name them.
- **`ui/`** is the app, on the published [`@withworkspace/ui-lib`](https://www.npmjs.com/package/@withworkspace/ui-lib): two screens (`/` and `/things/<id>`), a title for each, and a panel of what ws has told it.
- **`ui/dist/`** is the app as built, and is committed: `go:embed` needs it when the plugin's built, so Go alone builds the plugin.

## Running it

In a throwaway workspace, on this checkout's build:

```sh
just try up frame-fixture
```

That adds it, linked to this folder, and prints where the web UI is. In any other workspace: `ws plugin add <this folder> --link --build`.

## What to look for

| Where | What |
|---|---|
| `/x/frame-fixture/` | The app, in ws's theme from its first paint, light and dark. Its panel says frame protocol 1. |
| `/x/frame-fixture/things/2` | A deep link, loaded fresh: the app starts there, and the tab's title is "Lantern". |
| A thing's link, then back | The address and the title follow, and "ws:navigate received" goes up each time: ws keeps the history, and tells the app. One click is one step back. |
| ⌘-click a thing's link | It opens in a tab of its own at `/x/frame-fixture/things/<id>`, ws's page, not the bare shell. |
| "a relative link", ⌘-clicked | The tab asks for the bare shell's address, and is sent on to `/x/frame-fixture/things/3`. |
| "ws.go to thing 1", "Filter (replace)" | The first adds a step to back; the filter puts its address in place of the one it was on. |
| "a place in this page" | The page scrolls, and back has no new step. |
| "Frame fixture" and "Fixture: thing 2" in the sidebar | ws moves the app without loading it again: "ws:navigate received" goes up. |
| The theme switch, and the accent in Settings | The app follows: "ws:theme received" goes up, and the swatches change. |
| `ws frame-fixture poke` | "ws:changed received" goes up, and the app fetches the plugin's count again. |
| `?`, `g` then `m`, ⌘K, pressed in the frame | ws's keys work there. Typed into the box, `?` and `g` stay in the box. |
| "Missions: a page of ws's own" | A plain link to a page of ws's opens in ws, not in the frame. |
| `/x/frame-fixture/?speaks=2` | The app says it speaks only frame protocol 2. ws refuses it and says "Update ws" above the frame, and the package's `ProtocolError` is in the console. |

`FRAME_FIXTURE_SPEAKS=2` in the daemon's environment does the same as `?speaks=2` for every load, without the address saying so.

`ws plugin check <this folder>` says what it registers, with its `ui` line, and that nothing's wrong with it.

## Changing the app

```sh
cd ui
pnpm install
pnpm build      # type-checks, then writes dist/app.js and dist/app.css
```

Commit `ui/dist/` with the change. Then, in the try's workspace, `ws plugin add <this folder> --link --build` builds the plugin with the new app and starts it again.
