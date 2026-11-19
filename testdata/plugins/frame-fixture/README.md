# frame-fixture

A plugin for trying ws's page for a plugin's app (the shell) and the frame protocol by hand. It isn't published, and it isn't built on the plugin framework: it speaks the plugin protocol's JSON-RPC by hand (`main.go`), so ws can be tried before the framework builds apps itself.

- **`main.go`** registers two views and its app (`ui.script`, `ui.style`), and serves the app's files at `/ui/` and one JSON endpoint, `/api/things`. It serves no HTML. It registers how its things are named too (`refs`: `THING-2` is the Lantern), answers `ref/show` for one, and tells ws of all three as it starts (`thing/put`), so Find has them: `just try up plugin-refs` seeds a mission and a note that name them.
- **`ui/`** is the app, on the published [`@withworkspace/ui-lib`](https://www.npmjs.com/package/@withworkspace/ui-lib): two screens (`/` and `/things/<id>`), a title for each, some words of its own that name things, a menu and two dialogs, and a panel of what ws has told it.
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
| The top of `/x/frame-fixture/` | ws's glow carries on behind the app: the frame's page is see-through. `/plugin-frame/frame-fixture/` opened alone is plain `--ws-bg`. |
| `/x/frame-fixture/things/2` | A deep link, loaded fresh: the app starts there, and the tab's title is "Lantern". |
| A thing's link, then back | The address and the title follow, and "ws:navigate received" goes up each time: ws keeps the history, and tells the app. One click is one step back. |
| ⌘-click a thing's link | It opens in a tab of its own at `/x/frame-fixture/things/<id>`, ws's page, not the bare shell. |
| "a relative link", ⌘-clicked | The tab asks for the bare shell's address, and is sent on to `/x/frame-fixture/things/3`. |
| "ws.go to thing 1", "Filter (replace)" | The first adds a step to back; the filter puts its address in place of the one it was on. |
| "a place in this page" | The page scrolls, and back has no new step. |
| "Frame fixture" and "Fixture: thing 2" in the sidebar | ws moves the app without loading it again: "ws:navigate received" goes up. |
| The theme switch, and the accent in Settings | The app follows: "ws:theme received" goes up, and the swatches change. |
| `ws frame-fixture poke` | "ws:changed received" goes up, and the app fetches the plugin's count again. |
| `ws frame-fixture looking` | Who ws last told the plugin is looking (`view/looking`): `{"app":true,"ws":true}` with the app on screen, `app` false on another page of ws's, and both false a few seconds after the tab's put behind another, or closed. |
| `ws frame-fixture said THING-2` | What ws shows of a ref, as it answers a plugin that asks (`show`): what it is, what it links to, and what's said of it in the workspace, as JSON. `said <mission> <ref>` is one of a mission's own. |
| `ws frame-fixture talk "Let's shelve THING-2"` | ws starts a conversation with that as its first message, as it does for a plugin that asks (`conversation/start`), and prints the conversation and its page. The conversation says it was started from the plugin, and `ws show THING-2` then says the conversation mentions it. |
| `ws plugin config frame-fixture shelves.top.label=Top`, then `ws frame-fixture shelves` | Config in a table: the fixture declares `shelves`, named rows with a label, how many each holds, and its hooks (rows in turn). `ws plugin config frame-fixture` lists them, a field is set by its path, `--ask` asks a row at a time, and `shelves` prints the rows as ws last gave them, in the order they were written. |
| `?`, `g` then `m`, ⌘K, pressed in the frame | ws's keys work there. Typed into the box, `?` and `g` stay in the box. |
| "Missions: a page of ws's own" | A plain link to a page of ws's opens in ws, not in the frame. |
| "Names in the app's own words" | The names are refs, drawn as in ws's own pages: `THING-2` and `THING-1`, `THING-3` in code, and `THING-9`, which has a name and isn't there. The app holds no grammar: it sent its words to ws, which found them (`ws:refs`). A click on `THING-1` goes to it in the app, by way of ws. The link's words, `ws show THING-3` and the words that only look like names are left. |
| The same, after `just try up plugin-refs` | `shelve-the-lantern` is a ref to the mission, and the note's path one to the file, a moment after the rest: ws read the repo's files once the words named it, and said `ws:refs-changed`. "ws:refs-changed received" goes up for each. `ws mission new "Another one"` then leaves the marks as they are and the count one higher. |
| "A menu", then a press on ws's sidebar | The menu closes, as it does for a press elsewhere in the app, and "ws:outside received" goes up: the app never sees a press in ws, so ws says it. A press on the menu's own button closes it too, and doesn't open it again. |
| "A dialog" | The frame dims behind it, and ws's sidebar dims with it, in the same scrim, up to a straight edge where the two meet: the glow round the sidebar's card dims too. Both fade in together and go together. Light and dark. On a narrow window it's the bar across the top that dims. A press on either dimmed side closes it. |
| "A dialog that stays", then a press on the dimmed sidebar | Nothing in the sidebar can be pressed, the dialog stays, and Escape still closes it: the press took no focus from the frame. ⌘K and back still work while it's up: the dim isn't a lock. |
| The same, then a link in ws's palette (⌘K) to another page | ws's side isn't left dimmed when the app's page is gone. |
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
