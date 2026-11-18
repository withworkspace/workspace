var __defProp = Object.defineProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};

// ../../../../../../ui-lib/frame-overlays/dist/mark.js
var skipped = "a, pre, h1, h2, h3, h4, h5, h6, script, style, textarea, select, button, svg, [contenteditable]";
var isElement = (n) => n.nodeType === 1;
var isText = (n) => n.nodeType === 3;
var drawn = /* @__PURE__ */ new WeakMap();
var marked = /* @__PURE__ */ new WeakMap();
function runsIn(root) {
  const runs = [];
  const walk = (parent, whole) => {
    let run;
    for (const node of Array.from(parent.childNodes)) {
      if (isText(node) || isElement(node) && drawn.has(node)) {
        if (!run)
          runs.push(run = { nodes: [], text: "", whole });
        run.nodes.push(node);
        run.text += node.textContent ?? "";
        continue;
      }
      run = void 0;
      if (isElement(node) && !node.matches(skipped))
        walk(node, whole || node.localName === "code");
    }
  };
  walk(root, root.localName === "code");
  return runs.filter((r) => r.text.trim() !== "");
}
var drawnFrom = (f) => JSON.stringify([f.ref, f.kind, f.href, f.shape ?? "", f.accent ?? ""]);
function toDraw(run, found) {
  const out = [];
  let at = 0;
  for (const f of found) {
    if (f.from < at || f.to <= f.from || f.to > run.text.length)
      continue;
    out.push(f);
    at = f.to;
  }
  if (!run.whole)
    return out;
  const start = run.text.length - run.text.trimStart().length;
  const end = run.text.trimEnd().length;
  return out.length === 1 && out[0].from === start && out[0].to === end ? out : [];
}
function isDrawn(run, refs) {
  const have = [];
  let at = 0;
  for (const n of run.nodes) {
    const length = (n.textContent ?? "").length;
    if (isElement(n))
      have.push(`${at}-${at + length} ${drawn.get(n)}`);
    at += length;
  }
  const want = refs.map((f) => `${f.from}-${f.to} ${drawnFrom(f)}`);
  return have.length === want.length && have.every((h2, i) => h2 === want[i]);
}
function link(doc, f, text) {
  const a = doc.createElement("a");
  a.className = "ws-ref";
  a.setAttribute("href", f.href);
  a.dataset.wsRef = f.ref;
  a.dataset.wsKind = f.kind;
  if (f.shape)
    a.dataset.wsShape = f.shape;
  if (f.accent)
    a.style.setProperty("--ws-ref-accent", f.accent);
  a.textContent = text;
  drawn.set(a, drawnFrom(f));
  return a;
}
function draw(run, found) {
  const first = run.nodes[0];
  const parent = first?.parentNode;
  if (!first || !parent)
    return;
  const asRead = run.nodes.every((n, i) => n.parentNode === parent && (i === 0 || run.nodes[i - 1].nextSibling === n));
  if (!asRead || run.nodes.map((n) => n.textContent ?? "").join("") !== run.text)
    return;
  const refs = toDraw(run, found);
  if (isDrawn(run, refs))
    return;
  const doc = parent.ownerDocument;
  if (!doc)
    return;
  const words = doc.createDocumentFragment();
  let at = 0;
  for (const f of refs) {
    if (f.from > at)
      words.append(doc.createTextNode(run.text.slice(at, f.from)));
    words.append(link(doc, f, run.text.slice(f.from, f.to)));
    at = f.to;
  }
  if (at < run.text.length)
    words.append(doc.createTextNode(run.text.slice(at)));
  parent.insertBefore(words, first);
  for (const n of run.nodes)
    n.remove();
}
function mark(el, finder) {
  const was = marked.get(el);
  if (was) {
    was.again();
    return was.stop;
  }
  let stopped = false;
  let turn = 0;
  const again = () => {
    if (stopped)
      return;
    const runs = runsIn(el);
    const mine = ++turn;
    if (!runs.length)
      return;
    void finder.find(runs.map((r) => r.text)).then((found) => {
      if (stopped || mine !== turn)
        return;
      runs.forEach((run, i) => draw(run, found[i] ?? []));
    });
  };
  const unwatch = finder.onChanged(again);
  const stop = () => {
    if (stopped)
      return;
    stopped = true;
    unwatch();
    marked.delete(el);
    for (const run of runsIn(el))
      draw(run, []);
  };
  marked.set(el, { again, stop });
  again();
  return stop;
}

// ../../../../../../ui-lib/frame-overlays/dist/shape.js
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

// ../../../../../../ui-lib/frame-overlays/dist/v1.js
var v1_exports = {};
__export(v1_exports, {
  isFoundRef: () => isFoundRef,
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
  return isRecord(m) && m.type === "ws:welcome" && m.version === 1 && isString(m.ws) && isPlugin(m.plugin) && isString(m.path) && isTheme(m.theme) && isKeys(m.keys) && (m.refs === void 0 || typeof m.refs === "boolean");
}
var isIndex = (v) => Number.isInteger(v) && v >= 0;
function isFoundRef(m) {
  return isRecord(m) && isIndex(m.from) && isIndex(m.to) && m.to > m.from && isString(m.ref) && isString(m.kind) && isString(m.href) && (m.shape === void 0 || isString(m.shape)) && (m.accent === void 0 || isString(m.accent));
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
    case "ws:refs":
      return isIndex(m.id) && isStrings(m.texts);
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
    case "ws:refs-found":
      return isIndex(m.id) && Array.isArray(m.found) && m.found.every((of) => Array.isArray(of) && of.every(isFoundRef));
    case "ws:refs-changed":
      return true;
    case "ws:outside":
      return true;
    default:
      return false;
  }
}

// ../../../../../../ui-lib/frame-overlays/dist/protocol.js
var PROTOCOL_VERSIONS = [1];
function isRefuse(m) {
  return isRecord(m) && m.type === "ws:refuse" && isVersions(m.versions) && isString(m.message);
}

// ../../../../../../ui-lib/frame-overlays/dist/version.js
var VERSION = "0.0.0";

// ../../../../../../ui-lib/frame-overlays/dist/client.js
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
  const refsChanged = /* @__PURE__ */ new Set();
  const outside = /* @__PURE__ */ new Set();
  const marks = /* @__PURE__ */ new Set();
  const dismissals = /* @__PURE__ */ new Set();
  const asked = /* @__PURE__ */ new Map();
  let asking = [];
  let askings = 0;
  let closed = false;
  const send = (m) => port.postMessage(m);
  const on = (fns, fn) => {
    fns.add(fn);
    return () => {
      fns.delete(fn);
    };
  };
  const { at, base } = welcome.plugin;
  const open = (to, replace) => send(replace ? { type: "ws:go", path: to, replace } : { type: "ws:go", path: to });
  const ask = () => {
    const batch = asking;
    asking = [];
    if (!batch.length)
      return;
    const id = ++askings;
    asked.set(id, (found) => {
      let at2 = 0;
      for (const b of batch) {
        b.found(b.texts.map((_, i) => found[at2 + i] ?? []));
        at2 += b.texts.length;
      }
    });
    send({ type: "ws:refs", id, texts: batch.flatMap((b) => [...b.texts]) });
  };
  const findRefs = (texts) => new Promise((resolve) => {
    if (!welcome.refs || closed || !texts.length)
      return resolve(texts.map(() => []));
    if (!asking.length)
      queueMicrotask(ask);
    asking.push({ texts: [...texts], found: resolve });
  });
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
      case "ws:refs-found": {
        const found = asked.get(m.id);
        asked.delete(m.id);
        found?.(m.found);
        break;
      }
      case "ws:refs-changed":
        for (const fn of [...refsChanged])
          fn();
        break;
      case "ws:outside":
        for (const fn of [...outside])
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
    findRefs,
    onRefsChanged: (fn) => on(refsChanged, fn),
    markRefs(element) {
      const stop = mark(element, { find: (texts) => findRefs(texts), onChanged: (fn) => on(refsChanged, fn) });
      marks.add(stop);
      return () => {
        marks.delete(stop);
        stop();
      };
    },
    onOutside: (fn) => on(outside, fn),
    dismissOutside(within, fn) {
      const els = "nodeType" in within ? [within] : within;
      const pressed = (e) => {
        const on2 = e.target;
        if (on2 && els.some((el) => el.contains(on2)))
          return;
        fn();
      };
      doc?.addEventListener("pointerdown", pressed, true);
      const unheard = on(outside, () => fn());
      const stop = () => {
        dismissals.delete(stop);
        doc?.removeEventListener("pointerdown", pressed, true);
        unheard();
      };
      dismissals.add(stop);
      return stop;
    },
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
      for (const stop of [...marks])
        stop();
      marks.clear();
      for (const stop of [...dismissals])
        stop();
      closed = true;
      for (const b of asking.splice(0))
        b.found(b.texts.map(() => []));
      for (const found of [...asked.values()])
        found([]);
      asked.clear();
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
  const counts = { changed: 0, themes: 0, navigated: 0, refs: 0, outside: 0 };
  let closeMenu = () => {
  };
  function menu(button) {
    const items = ["Rename", "Move", "Put back on the shelf"].map((name) => h("button", { type: "button", role: "menuitem", onclick: () => closeMenu() }, name));
    const list = h("div", { className: "menu", role: "menu", data: { fixture: "menu" } }, ...items);
    const at = button.getBoundingClientRect();
    list.style.left = `${at.left}px`;
    list.style.top = `${at.bottom + 4}px`;
    document.body.append(list);
    const stop = ws.dismissOutside([button, list], () => closeMenu());
    closeMenu = () => {
      stop();
      list.remove();
      closeMenu = () => {
      };
    };
  }
  let closeDialog = () => {
  };
  function dialog(light) {
    closeDialog();
    const box = h(
      "div",
      { className: "dialog", role: "dialog", data: { fixture: "dialog" } },
      h("h1", {}, light ? "A dialog a press elsewhere closes" : "A dialog that stays"),
      h("p", {}, light ? "Press the dimmed frame, or the dimmed sidebar: either closes it." : "Press the dimmed sidebar: it stays, and Escape still closes it, since the press took no focus from the frame."),
      h("input", { placeholder: "Typed in here" }),
      h("p", {}, h("button", { type: "button", className: "primary", onclick: () => closeDialog() }, "Close"))
    );
    box.setAttribute("aria-modal", "true");
    const backdrop = h("div", { className: "ws-backdrop" }, box);
    document.body.append(backdrop);
    box.querySelector("input")?.focus();
    const stop = light ? ws.dismissOutside(box, () => closeDialog()) : () => {
    };
    const onKey = (e) => {
      if (e.key === "Escape") closeDialog();
    };
    document.addEventListener("keydown", onKey);
    closeDialog = () => {
      stop();
      document.removeEventListener("keydown", onKey);
      backdrop.remove();
      closeDialog = () => {
      };
    };
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
    const one = /^\/things\/(?:THING-)?(\d+)$/.exec(path);
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
  function names() {
    const words = h("div", { className: "names" });
    words.innerHTML = `
      <h2>Names in the app's own words</h2>
      <p>THING-2 goes back on the shelf, beside THING-1. The mission is shelve-the-lantern, and its note is records/notes/where-the-things-are.md, in records.</p>
      <p><code>THING-3</code> is a name in code, and <code>ws show THING-3</code> isn't one. <a href="${ws.href("/things/1")}">A link's words, THING-1 among them,</a> are the link's. THING-9 has a name and isn't there; THING-2-fix and docs/THING-2.md aren't names at all.</p>`;
    return words;
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
        row("ws:navigate received", counts.navigated, "navigated"),
        row("ws:refs-changed received", counts.refs, "refs"),
        row("ws:outside received", counts.outside, "outside")
      ),
      h("p", { className: "swatches" }, ...["bg", "panel", "raised", "line", "fg", "dim", "accent", "primary", "danger", "warn", "ok"].map(swatch)),
      h("p", {}, h("button", { type: "button", className: "primary" }, "A button in ws's accent"), " ", h("input", { placeholder: "Typed in here, ? and g stay here" })),
      h("p", {}, h("a", { href: "/" }, "Missions: a page of ws's own"), " \xB7 ", h("a", { href: "things/3" }, "a relative link"), " \xB7 ", h("a", { href: "#notes" }, "a place in this page")),
      h("p", {}, h("button", { type: "button", data: { fixture: "go" }, onclick: () => ws.go("/things/1") }, "ws.go to thing 1"), " ", h("button", { type: "button", data: { fixture: "replace" }, onclick: () => ws.go(`${ws.path.split("?")[0]}?filtered=${Date.now() % 1e3}`, { replace: true }) }, "Filter (replace)")),
      h(
        "p",
        {},
        h("button", { type: "button", data: { fixture: "menu-button" }, onclick: (e) => document.querySelector(".menu") ? closeMenu() : menu(e.currentTarget) }, "A menu"),
        " ",
        h("button", { type: "button", data: { fixture: "light-dialog" }, onclick: () => dialog(true) }, "A dialog"),
        " ",
        h("button", { type: "button", data: { fixture: "staying-dialog" }, onclick: () => dialog(false) }, "A dialog that stays")
      ),
      h("p", { id: "notes", className: "dim" }, "Notes: back after a filter skips it, as it replaced the entry it was on.")
    );
  }
  let unmark = () => {
  };
  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    closeMenu();
    unmark();
    const words = names();
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body, words), status()));
    unmark = ws.markRefs(words);
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
  ws.onRefsChanged(() => {
    counts.refs++;
    const cell = ws.root.querySelector('[data-fixture="refs"]');
    if (cell) cell.textContent = String(counts.refs);
  });
  ws.onOutside(() => {
    counts.outside++;
    const cell = ws.root.querySelector('[data-fixture="outside"]');
    if (cell) cell.textContent = String(counts.outside);
  });
  render();
  await load();
}
void main();
