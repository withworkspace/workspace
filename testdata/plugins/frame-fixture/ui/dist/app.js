var __defProp = Object.defineProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};

// node_modules/.pnpm/@withworkspace+ui-lib@0.1.0/node_modules/@withworkspace/ui-lib/dist/shape.js
function isRecord(v) {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
function isString(v) {
  return typeof v === "string";
}
function isStrings(v) {
  return Array.isArray(v) && v.every(isString);
}
function isVersions(v) {
  return Array.isArray(v) && v.every((n) => Number.isInteger(n) && n >= 1);
}

// node_modules/.pnpm/@withworkspace+ui-lib@0.1.0/node_modules/@withworkspace/ui-lib/dist/v1.js
var v1_exports = {};
__export(v1_exports, {
  isHostMessage: () => isHostMessage,
  isPageMessage: () => isPageMessage,
  isWelcome: () => isWelcome
});
function isTheme(m) {
  return isRecord(m) && (m.scheme === "light" || m.scheme === "dark") && isRecord(m.vars) && Object.values(m.vars).every(isString);
}
function isKeys(m) {
  return isRecord(m) && isStrings(m.chords) && isStrings(m.bare) && isStrings(m.leaders) && typeof m.within === "number";
}
function isPlugin(m) {
  return isRecord(m) && isString(m.name) && isString(m.base) && isString(m.api);
}
function isWelcome(m) {
  return isRecord(m) && m.type === "ws:welcome" && m.version === 1 && isString(m.ws) && isPlugin(m.plugin) && isString(m.path) && isTheme(m.theme) && isKeys(m.keys);
}
function isPageMessage(m) {
  if (!isRecord(m))
    return false;
  switch (m.type) {
    case "ws:path":
    case "ws:open":
      return isString(m.path);
    case "ws:title":
      return isString(m.title);
    case "ws:key":
      return isString(m.key) && [m.meta, m.ctrl, m.shift, m.alt].every((v) => typeof v === "boolean");
    default:
      return false;
  }
}
function isHostMessage(m) {
  if (!isRecord(m))
    return false;
  switch (m.type) {
    case "ws:welcome":
      return isWelcome(m);
    case "ws:refuse":
      return isVersions(m.versions) && isString(m.message);
    case "ws:navigate":
      return isString(m.path);
    case "ws:theme":
      return isTheme(m);
    case "ws:changed":
      return true;
    default:
      return false;
  }
}

// node_modules/.pnpm/@withworkspace+ui-lib@0.1.0/node_modules/@withworkspace/ui-lib/dist/protocol.js
var PROTOCOL_VERSIONS = [1];
function isRefuse(m) {
  return isRecord(m) && m.type === "ws:refuse" && isVersions(m.versions) && isString(m.message);
}

// node_modules/.pnpm/@withworkspace+ui-lib@0.1.0/node_modules/@withworkspace/ui-lib/dist/version.js
var VERSION = "0.1.0";

// node_modules/.pnpm/@withworkspace+ui-lib@0.1.0/node_modules/@withworkspace/ui-lib/dist/client.js
var ProtocolError = class extends Error {
  /** The versions the page speaks. */
  page;
  /** The versions ws said it speaks. */
  host;
  constructor(message, page, host) {
    super(message);
    this.name = "ProtocolError";
    this.page = page;
    this.host = host;
  }
};
var NoHostError = class extends Error {
  constructor(message, options2) {
    super(message, options2);
    this.name = "NoHostError";
  }
};
function connect(options2 = {}) {
  const doc = options2.document ?? (typeof document === "undefined" ? void 0 : document);
  const view = doc?.defaultView ?? void 0;
  const timeout = options2.timeout ?? 5e3;
  let host = options2.host;
  if (!host) {
    if (!view || view.parent === view) {
      return Promise.reject(new NoHostError("This page isn't in a frame, so there's no ws to connect to. Open it in ws, at /x/<plugin>/."));
    }
    host = view.parent;
  }
  const origin = view?.location?.origin;
  const target = origin && origin !== "null" ? origin : "/";
  const versions = [...PROTOCOL_VERSIONS];
  const hello = { type: "ws:connect", versions, client: `@withworkspace/ui-lib@${VERSION}` };
  return new Promise((resolve, reject) => {
    const { port1: port, port2 } = new MessageChannel();
    const fail = (err) => {
      clearTimeout(timer);
      port.onmessage = null;
      port.close();
      reject(err);
    };
    const refuse = (err) => {
      console.error(err);
      fail(err);
    };
    const timer = setTimeout(() => {
      fail(new NoHostError(`ws didn't answer within ${timeout}ms. This page isn't in ws's frame, or it's in a ws from before frame protocol ${Math.min(...versions)}: update ws.`));
    }, timeout);
    port.onmessage = (e) => {
      const m = e.data;
      if (isRefuse(m)) {
        refuse(new ProtocolError(m.message, versions, m.versions));
      } else if (v1_exports.isWelcome(m)) {
        clearTimeout(timer);
        resolve(client(port, m, doc));
      } else if (isRecord(m) && m.type === "ws:welcome") {
        const said = typeof m.version === "number" ? [m.version] : [];
        refuse(new ProtocolError(`ws welcomed this page in frame protocol ${String(m.version)}, and not as this page reads it: it speaks ${versions.join(", ")}.`, versions, said));
      }
    };
    try {
      host.postMessage(hello, target, [port2]);
    } catch (err) {
      fail(new NoHostError("ws:connect couldn't be sent to the page's parent.", { cause: err }));
    }
  });
}
function client(port, welcome, doc) {
  let path = welcome.path;
  let theme = welcome.theme;
  const navigated = /* @__PURE__ */ new Set();
  const changed = /* @__PURE__ */ new Set();
  const themed = /* @__PURE__ */ new Set();
  const send = (m) => port.postMessage(m);
  const on = (fns, fn) => {
    fns.add(fn);
    return () => {
      fns.delete(fn);
    };
  };
  const open = (to) => send({ type: "ws:open", path: to });
  if (doc)
    applyTheme(doc, theme);
  const unwire = doc ? wire(doc, welcome.keys, send, open) : () => {
  };
  port.onmessage = (e) => {
    const m = e.data;
    if (!v1_exports.isHostMessage(m))
      return;
    switch (m.type) {
      case "ws:navigate":
        path = m.path;
        for (const fn of [...navigated])
          fn(m.path);
        break;
      case "ws:theme":
        theme = { scheme: m.scheme, vars: m.vars };
        if (doc)
          applyTheme(doc, theme);
        for (const fn of [...themed])
          fn(theme);
        break;
      case "ws:changed":
        for (const fn of [...changed])
          fn();
        break;
    }
  };
  return {
    version: welcome.version,
    name: welcome.plugin.name,
    base: welcome.plugin.base,
    get root() {
      const root = doc?.getElementById("root");
      if (!root)
        throw new Error(doc ? `There's no <div id="root"> in the page: ws's shell has one.` : "There's no document, so there's no #root: give connect() one.");
      return root;
    },
    get path() {
      return path;
    },
    get theme() {
      return theme;
    },
    api: (p) => welcome.plugin.api + (p.startsWith("/") ? p : `/${p}`),
    onNavigate: (fn) => on(navigated, fn),
    onChanged: (fn) => on(changed, fn),
    onTheme: (fn) => on(themed, fn),
    setPath(to) {
      if (!to.startsWith("/"))
        throw new TypeError(`setPath("${to}"): a path in the plugin starts with "/".`);
      if (to === path)
        return;
      path = to;
      send({ type: "ws:path", path: to });
    },
    setTitle(title) {
      if (doc)
        doc.title = title;
      send({ type: "ws:title", title });
    },
    open,
    close() {
      unwire();
      port.onmessage = null;
      port.close();
    }
  };
}
function applyTheme(doc, theme) {
  const root = doc.documentElement;
  root.dataset.theme = theme.scheme;
  root.style.colorScheme = theme.scheme;
  for (const [name, value] of Object.entries(theme.vars))
    root.style.setProperty(name, value);
}
function wire(doc, keys, send, open) {
  let leaderAt = 0;
  const onKey = (e) => {
    const typing = isTypedIn(e.target);
    const chord = (e.metaKey || e.ctrlKey) && keys.chords.includes(e.key);
    const plain = !typing && !e.metaKey && !e.ctrlKey && !e.altKey;
    const after = plain && Date.now() - leaderAt < keys.within && /^[a-z]$/.test(e.key);
    const bare = plain && (keys.bare.includes(e.key) || after);
    if (bare)
      leaderAt = keys.leaders.includes(e.key) ? Date.now() : 0;
    if (!chord && !bare)
      return;
    e.preventDefault();
    send({ type: "ws:key", key: e.key, meta: e.metaKey, ctrl: e.ctrlKey, shift: e.shiftKey, alt: e.altKey });
  };
  const onClick = (e) => {
    const target = e.target;
    const a = target?.closest?.("a[data-ws]");
    if (!a || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0)
      return;
    e.preventDefault();
    open(a.getAttribute("data-ws") ?? "");
  };
  doc.addEventListener("keydown", onKey);
  doc.addEventListener("click", onClick);
  return () => {
    doc.removeEventListener("keydown", onKey);
    doc.removeEventListener("click", onClick);
  };
}
function isTypedIn(target) {
  const el = target;
  return !!el && (el.isContentEditable === true || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName ?? ""));
}

// src/main.ts
function h(tag, props = {}, ...children) {
  const { data, ...rest } = props;
  const el = Object.assign(document.createElement(tag), rest);
  for (const [k, v] of Object.entries(data ?? {})) el.dataset[k] = v;
  el.append(...children);
  return el;
}
var speaks = Number(new URLSearchParams(location.search).get("speaks") ?? new URL(import.meta.url).searchParams.get("speaks")) || void 0;
var options = speaks ? { host: { postMessage: (message, origin, transfer) => window.parent.postMessage({ ...message, versions: [speaks] }, origin, transfer) } } : {};
async function main() {
  let ws;
  try {
    ws = await connect(options);
  } catch (err) {
    const root = document.getElementById("root") ?? document.body;
    const why = err instanceof ProtocolError ? `This page speaks frame protocol ${err.page.join(", ")}${speaks ? ` (and said ${speaks})` : ""}, and ws speaks ${err.host.join(", ")}.` : err instanceof NoHostError ? "Nothing answered: this page is for ws's frame, at /x/frame-fixture/." : "";
    root.replaceChildren(h("main", { className: "failed" }, h("h1", {}, "The fixture couldn't connect to ws"), h("p", {}, err instanceof Error ? err.message : String(err)), h("p", { className: "dim" }, why)));
    return;
  }
  let things;
  let poked = 0;
  let failed = "";
  const counts = { changed: 0, themes: 0, navigated: 0 };
  const here = () => (location.pathname.slice(ws.base.length) || "/") + location.search;
  function go(path) {
    history.pushState(null, "", ws.base + path);
    ws.setPath(path);
    render();
  }
  async function load() {
    try {
      const res = await fetch(ws.api("/api/things"));
      if (!res.ok) throw new Error(`${ws.api("/api/things")} answered ${res.status}`);
      const body = await res.json();
      [things, poked, failed] = [body.things, body.poked, ""];
    } catch (err) {
      failed = err instanceof Error ? err.message : String(err);
    }
    render();
  }
  function screen() {
    const path = ws.path.split("?")[0];
    const one = /^\/things\/(\d+)$/.exec(path);
    if (path === "/") {
      const list = things?.map((t) => h("li", {}, h("a", { href: `${ws.base}/things/${t.id}` }, t.name), " ", h("span", { className: "dim" }, t.note)));
      return { title: "Things", body: [h("h1", {}, "Things"), failed ? h("p", { className: "error" }, failed) : list ? h("ul", { className: "things" }, ...list) : h("p", { className: "dim" }, "Loading\u2026")] };
    }
    if (one) {
      const t = things?.find((t2) => t2.id === Number(one[1]));
      const back = h("p", {}, h("a", { href: `${ws.base}/` }, "\u2190 All things"));
      if (!things && !failed) return { title: `Thing ${one[1]}`, body: [back, h("p", { className: "dim" }, "Loading\u2026")] };
      if (!t) return { title: `No thing ${one[1]}`, body: [back, h("h1", {}, `There's no thing ${one[1]}`), failed ? h("p", { className: "error" }, failed) : ""] };
      return { title: t.name, body: [back, h("h1", {}, t.name), h("p", {}, t.note)] };
    }
    return { title: "Nothing here", body: [h("h1", {}, "Nothing here"), h("p", {}, `The fixture has no screen at ${path}. `, h("a", { href: `${ws.base}/` }, "All things"))] };
  }
  function status() {
    const row = (name, value, id) => h("tr", {}, h("th", {}, name), h("td", { data: { fixture: id } }, String(value)));
    const swatch = (token) => {
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
        row("ws:navigate received", counts.navigated, "navigated")
      ),
      h("p", { className: "swatches" }, ...["bg", "panel", "raised", "line", "fg", "dim", "accent", "primary", "danger", "warn", "ok"].map(swatch)),
      h("p", {}, h("button", { type: "button", className: "primary" }, "A button in ws's accent"), " ", h("input", { placeholder: "Typed in here, ? and g stay here" })),
      h("p", {}, h("a", { href: "/", data: { ws: "/" } }, "Missions: a page of ws's own (data-ws)"))
    );
  }
  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body), status()));
  }
  ws.root.addEventListener("click", (e) => {
    const a = e.target.closest?.("a[href]");
    if (!(a instanceof HTMLAnchorElement) || a.dataset.ws !== void 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    const path = a.pathname + a.search;
    if (a.origin !== location.origin || !(path === ws.base || path.startsWith(`${ws.base}/`))) return;
    e.preventDefault();
    go(path.slice(ws.base.length) || "/");
  });
  window.addEventListener("popstate", () => {
    ws.setPath(here());
    render();
  });
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
  if (here() !== ws.path) history.replaceState(null, "", ws.base + ws.path);
  render();
  await load();
}
void main();
