// The fixture's app: what a plugin's app is, at its smallest, on the
// published @withworkspace/ui-lib. ws serves the page this runs in (the
// shell); this connects to ws, renders into #root, shows one of two
// screens for the path ws says it's at, says its title for each, and counts
// the ws:changed it's had. It keeps no history: ws does. The names in its
// own words (THING-2, a mission) are ws's to find: it marks them.

import { type Client, connect, type ConnectOptions, NoHostError, ProtocolError } from "@withworkspace/ui-lib";

interface Thing {
  id: number;
  name: string;
  note: string;
}

// h makes an element: its properties, then what's in it.
function h<K extends keyof HTMLElementTagNameMap>(tag: K, props: Partial<HTMLElementTagNameMap[K]> & { data?: Record<string, string> } = {}, ...children: (Node | string)[]): HTMLElementTagNameMap[K] {
  const { data, ...rest } = props;
  const el = Object.assign(document.createElement(tag), rest);
  for (const [k, v] of Object.entries(data ?? {})) el.dataset[k] = v;
  el.append(...children);
  return el;
}

// ?speaks=2, on the page's address or on this script's (the plugin puts it
// there when FRAME_FIXTURE_SPEAKS is set), makes this page say it speaks
// only that frame protocol version, to see ws refuse it. The package
// always says the versions it really speaks, so its ws:connect is changed
// on the way out: everything else is the package's own.
const speaks = Number(new URLSearchParams(location.search).get("speaks") ?? new URL(import.meta.url).searchParams.get("speaks")) || undefined;
const options: ConnectOptions = speaks
  ? { host: { postMessage: (message, origin, transfer) => window.parent.postMessage({ ...(message as object), versions: [speaks] }, origin, transfer) } }
  : {};

async function main() {
  let ws: Client;
  try {
    ws = await connect(options);
  } catch (err) {
    // The shell's #root, by hand: there's no client to give it.
    const root = document.getElementById("root") ?? document.body;
    const why =
      err instanceof ProtocolError
        ? `This page speaks frame protocol ${err.page.join(", ")}${speaks ? ` (and said ${speaks})` : ""}, and ws speaks ${err.host.join(", ")}.`
        : err instanceof NoHostError
          ? "Nothing answered: this page is for ws's frame, at /x/frame-fixture/."
          : "";
    root.replaceChildren(h("main", { className: "failed" }, h("h1", {}, "The fixture couldn't connect to ws"), h("p", {}, err instanceof Error ? err.message : String(err)), h("p", { className: "dim" }, why)));
    return;
  }

  let things: Thing[] | undefined;
  let poked = 0;
  let failed = "";
  const counts = { changed: 0, themes: 0, navigated: 0, refs: 0 };

  async function load() {
    try {
      const res = await fetch(ws.api("/api/things"));
      if (!res.ok) throw new Error(`${ws.api("/api/things")} answered ${res.status}`);
      const body = (await res.json()) as { things: Thing[]; poked: number };
      [things, poked, failed] = [body.things, body.poked, ""];
    } catch (err) {
      failed = err instanceof Error ? err.message : String(err);
    }
    render();
  }

  function screen(): { title: string; body: (Node | string)[] } {
    const path = ws.path.split("?")[0];
    // A thing by its number, or by its name as ws knows it (THING-2):
    // where a name written anywhere in ws opens.
    const one = /^\/things\/(?:THING-)?(\d+)$/.exec(path);
    if (path === "/") {
      const list = things?.map((t) => h("li", {}, h("a", { href: ws.href(`/things/${t.id}`) }, t.name), " ", h("span", { className: "dim" }, t.note)));
      return { title: "Things", body: [h("h1", {}, "Things"), failed ? h("p", { className: "error" }, failed) : list ? h("ul", { className: "things" }, ...list) : h("p", { className: "dim" }, "Loading…")] };
    }
    if (one) {
      const t = things?.find((t) => t.id === Number(one[1]));
      const back = h("p", {}, h("a", { href: ws.href("/") }, "← All things"));
      if (!things && !failed) return { title: `Thing ${one[1]}`, body: [back, h("p", { className: "dim" }, "Loading…")] };
      if (!t) return { title: `No thing ${one[1]}`, body: [back, h("h1", {}, `There's no thing ${one[1]}`), failed ? h("p", { className: "error" }, failed) : ""] };
      return { title: t.name, body: [back, h("h1", {}, t.name), h("p", {}, t.note)] };
    }
    return { title: "Nothing here", body: [h("h1", {}, "Nothing here"), h("p", {}, `The fixture has no screen at ${path}. `, h("a", { href: ws.href("/") }, "All things"))] };
  }

  // Words of the app's own that name things, set as HTML, as a
  // description from somewhere else would be: ws says which are names
  // (markRefs, in render), and they're drawn as refs are in ws.
  function names(): HTMLElement {
    const words = h("div", { className: "names" });
    words.innerHTML = `
      <h2>Names in the app's own words</h2>
      <p>THING-2 goes back on the shelf, beside THING-1. The mission is shelve-the-lantern, and its note is records/notes/where-the-things-are.md, in records.</p>
      <p><code>THING-3</code> is a name in code, and <code>ws show THING-3</code> isn't one. <a href="${ws.href("/things/1")}">A link's words, THING-1 among them,</a> are the link's. THING-9 has a name and isn't there; THING-2-fix and docs/THING-2.md aren't names at all.</p>`;
    return words;
  }

  // What ws has told this page, and what it's been given: for whoever's
  // trying the frame to read off.
  function status(): Node {
    const row = (name: string, value: string | number, id: string) => h("tr", {}, h("th", {}, name), h("td", { data: { fixture: id } }, String(value)));
    const swatch = (token: string) => {
      const colour = h("i");
      colour.style.background = `var(--ws-${token})`;
      return h("span", { className: "swatch", title: `--ws-${token}` }, colour, token);
    };
    return h(
      "aside",
      {},
      h("h2", {}, "What ws said"),
      h(
        "table",
        {},
        row("Frame protocol", ws.version, "version"),
        row("Path", ws.path, "path"),
        row("Theme", ws.theme.scheme, "scheme"),
        row("ws:changed received", counts.changed, "changed"),
        row("Pokes, as the plugin counts them", poked, "poked"),
        row("ws:theme received", counts.themes, "themes"),
        row("ws:navigate received", counts.navigated, "navigated"),
        row("ws:refs-changed received", counts.refs, "refs"),
      ),
      h("p", { className: "swatches" }, ...["bg", "panel", "raised", "line", "fg", "dim", "accent", "primary", "danger", "warn", "ok"].map(swatch)),
      h("p", {}, h("button", { type: "button", className: "primary" }, "A button in ws's accent"), " ", h("input", { placeholder: "Typed in here, ? and g stay here" })),
      h("p", {}, h("a", { href: "/" }, "Missions: a page of ws's own"), " · ", h("a", { href: "things/3" }, "a relative link"), " · ", h("a", { href: "#notes" }, "a place in this page")),
      h("p", {}, h("button", { type: "button", data: { fixture: "go" }, onclick: () => ws.go("/things/1") }, "ws.go to thing 1"), " ", h("button", { type: "button", data: { fixture: "replace" }, onclick: () => ws.go(`${ws.path.split("?")[0]}?filtered=${Date.now() % 1000}`, { replace: true }) }, "Filter (replace)")),
      h("p", { id: "notes", className: "dim" }, "Notes: back after a filter skips it, as it replaced the entry it was on."),
    );
  }

  // The marks of the words last rendered: stopped when they're replaced.
  let unmark = () => {};

  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    unmark();
    const words = names();
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body, words), status()));
    unmark = ws.markRefs(words);
  }

  // Wherever ws's address moves in the plugin (a link here, a go, the
  // sidebar, back and forward), the app shows it. Links need nothing of
  // the app's: the package hands each click to ws.
  ws.onNavigate(() => {
    counts.navigated++;
    render();
  });
  ws.onChanged(() => {
    counts.changed++;
    void load();
  });
  ws.onTheme(() => {
    counts.themes++;
    render();
  });
  // What there is to name changed: the marks follow by themselves, and
  // only the count is this app's to draw.
  ws.onRefsChanged(() => {
    counts.refs++;
    const cell = ws.root.querySelector('[data-fixture="refs"]');
    if (cell) cell.textContent = String(counts.refs);
  });

  render();
  await load();
}

void main();
