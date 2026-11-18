var __defProp = Object.defineProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};

// node_modules/.pnpm/@withworkspace+ui-lib@0.2.0/node_modules/@withworkspace/ui-lib/dist/shape.js
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

// node_modules/.pnpm/@withworkspace+ui-lib@0.2.0/node_modules/@withworkspace/ui-lib/dist/v1.js
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
  return isRecord(m) && isString(m.name) && isString(m.at) && isString(m.base) && isString(m.api);
}
function isWelcome(m) {
  return isRecord(m) && m.type === "ws:welcome" && m.version === 1 && isString(m.ws) && isPlugin(m.plugin) && isString(m.path) && isTheme(m.theme) && isKeys(m.keys);
}
function isPageMessage(m) {
  if (!isRecord(m))
    return false;
  switch (m.type) {
    case "ws:go":
      return isString(m.path) && (m.replace === void 0 || typeof m.replace === "boolean");
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

// node_modules/.pnpm/@withworkspace+ui-lib@0.2.0/node_modules/@withworkspace/ui-lib/dist/protocol.js
var PROTOCOL_VERSIONS = [1];
function isRefuse(m) {
  return isRecord(m) && m.type === "ws:refuse" && isVersions(m.versions) && isString(m.message);
}

// node_modules/.pnpm/@withworkspace+ui-lib@0.2.0/node_modules/@withworkspace/ui-lib/dist/version.js
var VERSION = "0.2.0";

// node_modules/.pnpm/@withworkspace+ui-lib@0.2.0/node_modules/@withworkspace/ui-lib/dist/client.js
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
  const { at, base } = welcome.plugin;
  const open = (to, replace) => send(replace ? { type: "ws:go", path: to, replace } : { type: "ws:go", path: to });
  if (doc)
    applyTheme(doc, theme);
  const unwire = doc ? wire(doc, welcome, send, open) : () => {
  };
  port.onmessage = (e) => {
    const m = e.data;
    if (!v1_exports.isHostMessage(m))
      return;
    switch (m.type) {
      case "ws:navigate":
        path = m.path;
        try {
          doc?.defaultView?.history.replaceState(doc.defaultView.history.state, "", base + m.path);
        } catch {
        }
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
    href: (p) => at + (p.startsWith("/") ? p : `/${p}`),
    onNavigate: (fn) => on(navigated, fn),
    onChanged: (fn) => on(changed, fn),
    onTheme: (fn) => on(themed, fn),
    go(to, options2 = {}) {
      if (!to.startsWith("/"))
        throw new TypeError(`go("${to}"): a path in the plugin starts with "/".`);
      open(at + to, options2.replace);
    },
    setTitle(title) {
      if (doc)
        doc.title = title;
      send({ type: "ws:title", title });
    },
    open: (to) => open(to),
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
function wire(doc, welcome, send, open) {
  const { keys } = welcome;
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
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)
      return;
    const target = e.target;
    const a = target?.closest?.("a[href]");
    if (!a || typeof a.href !== "string" || a.hasAttribute("download"))
      return;
    const named = a.getAttribute("target");
    if (named && named !== "_self")
      return;
    const view = doc.defaultView;
    if (!view)
      return;
    let url;
    let here;
    try {
      url = new URL(a.href, view.location.href);
      here = new URL(view.location.href);
    } catch {
      return;
    }
    if (url.protocol !== "http:" && url.protocol !== "https:")
      return;
    e.preventDefault();
    const api = welcome.plugin.api;
    if (url.origin !== here.origin || url.pathname === api || url.pathname.startsWith(`${api}/`)) {
      view.open(url.href, "_blank", "noopener");
      return;
    }
    if (url.hash && url.pathname === here.pathname && url.search === here.search) {
      doc.getElementById(decodeURIComponent(url.hash.slice(1)))?.scrollIntoView();
      return;
    }
    open(address(url, welcome.plugin));
  };
  doc.addEventListener("keydown", onKey);
  doc.addEventListener("click", onClick);
  return () => {
    doc.removeEventListener("keydown", onKey);
    doc.removeEventListener("click", onClick);
  };
}
function address(url, plugin) {
  const { pathname, search, hash } = url;
  const inFrame = pathname === plugin.base || pathname.startsWith(`${plugin.base}/`);
  const path = inFrame ? plugin.at + (pathname.slice(plugin.base.length) || "/") : pathname;
  return path + search + hash;
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
      const list = things?.map((t) => h("li", {}, h("a", { href: ws.href(`/things/${t.id}`) }, t.name), " ", h("span", { className: "dim" }, t.note)));
      return { title: "Things", body: [h("h1", {}, "Things"), failed ? h("p", { className: "error" }, failed) : list ? h("ul", { className: "things" }, ...list) : h("p", { className: "dim" }, "Loading\u2026")] };
    }
    if (one) {
      const t = things?.find((t2) => t2.id === Number(one[1]));
      const back = h("p", {}, h("a", { href: ws.href("/") }, "\u2190 All things"));
      if (!things && !failed) return { title: `Thing ${one[1]}`, body: [back, h("p", { className: "dim" }, "Loading\u2026")] };
      if (!t) return { title: `No thing ${one[1]}`, body: [back, h("h1", {}, `There's no thing ${one[1]}`), failed ? h("p", { className: "error" }, failed) : ""] };
      return { title: t.name, body: [back, h("h1", {}, t.name), h("p", {}, t.note)] };
    }
    return { title: "Nothing here", body: [h("h1", {}, "Nothing here"), h("p", {}, `The fixture has no screen at ${path}. `, h("a", { href: ws.href("/") }, "All things"))] };
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
      h("p", {}, h("a", { href: "/" }, "Missions: a page of ws's own"), " \xB7 ", h("a", { href: "things/3" }, "a relative link"), " \xB7 ", h("a", { href: "#notes" }, "a place in this page")),
      h("p", {}, h("button", { type: "button", data: { fixture: "go" }, onclick: () => ws.go("/things/1") }, "ws.go to thing 1"), " ", h("button", { type: "button", data: { fixture: "replace" }, onclick: () => ws.go(`${ws.path.split("?")[0]}?filtered=${Date.now() % 1e3}`, { replace: true }) }, "Filter (replace)")),
      h("p", { id: "notes", className: "dim" }, "Notes: back after a filter skips it, as it replaced the entry it was on.")
    );
  }
  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body), status()));
  }
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
  render();
  await load();
}
void main();
