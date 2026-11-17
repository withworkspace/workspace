// The fixture's app: what a plugin's app is, at its smallest, on the
// published @withworkspace/ui-lib. ws serves the page this runs in (the
// shell); this connects to ws, renders into #root, routes itself between
// two screens, says its title for each, and counts the ws:changed it's had.

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
  const counts = { changed: 0, themes: 0, navigated: 0 };

  // Where the frame's own address is, as a path in the plugin.
  const here = () => (location.pathname.slice(ws.base.length) || "/") + location.search;

  // The app moved, on its own: its own history gains an entry, and ws's
  // address follows.
  function go(path: string) {
    history.pushState(null, "", ws.base + path);
    ws.setPath(path);
    render();
  }

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
    const one = /^\/things\/(\d+)$/.exec(path);
    if (path === "/") {
      const list = things?.map((t) => h("li", {}, h("a", { href: `${ws.base}/things/${t.id}` }, t.name), " ", h("span", { className: "dim" }, t.note)));
      return { title: "Things", body: [h("h1", {}, "Things"), failed ? h("p", { className: "error" }, failed) : list ? h("ul", { className: "things" }, ...list) : h("p", { className: "dim" }, "Loading…")] };
    }
    if (one) {
      const t = things?.find((t) => t.id === Number(one[1]));
      const back = h("p", {}, h("a", { href: `${ws.base}/` }, "← All things"));
      if (!things && !failed) return { title: `Thing ${one[1]}`, body: [back, h("p", { className: "dim" }, "Loading…")] };
      if (!t) return { title: `No thing ${one[1]}`, body: [back, h("h1", {}, `There's no thing ${one[1]}`), failed ? h("p", { className: "error" }, failed) : ""] };
      return { title: t.name, body: [back, h("h1", {}, t.name), h("p", {}, t.note)] };
    }
    return { title: "Nothing here", body: [h("h1", {}, "Nothing here"), h("p", {}, `The fixture has no screen at ${path}. `, h("a", { href: `${ws.base}/` }, "All things"))] };
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
      ),
      h("p", { className: "swatches" }, ...["bg", "panel", "raised", "line", "fg", "dim", "accent", "primary", "danger", "warn", "ok"].map(swatch)),
      h("p", {}, h("button", { type: "button", className: "primary" }, "A button in ws's accent"), " ", h("input", { placeholder: "Typed in here, ? and g stay here" })),
      h("p", {}, h("a", { href: "/", data: { ws: "/" } }, "Missions: a page of ws's own (data-ws)")),
    );
  }

  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body), status()));
  }

  // A link to a screen of the app's own goes there without loading the
  // page again. (A data-ws link is the package's: it opens in ws.)
  ws.root.addEventListener("click", (e) => {
    const a = (e.target as Element).closest?.("a[href]");
    if (!(a instanceof HTMLAnchorElement) || a.dataset.ws !== undefined || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    const path = a.pathname + a.search;
    if (a.origin !== location.origin || !(path === ws.base || path.startsWith(`${ws.base}/`))) return;
    e.preventDefault();
    go(path.slice(ws.base.length) || "/");
  });
  // Back and forward in the frame's own history: ws's address follows.
  window.addEventListener("popstate", () => {
    ws.setPath(here());
    render();
  });
  // ws's address moved without the app: the sidebar, a link in ws, back
  // and forward there. The app goes there, replacing its own entry.
  ws.onNavigate((path) => {
    counts.navigated++;
    history.replaceState(null, "", ws.base + path);
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

  // The frame's own address starts where ws's is.
  if (here() !== ws.path) history.replaceState(null, "", ws.base + ws.path);
  render();
  await load();
}

void main();
