var __defProp = Object.defineProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};

// ../../../../../../ui-lib/frame-menu/dist/mark.js
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

// ../../../../../../ui-lib/frame-menu/dist/shape.js
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

// ../../../../../../ui-lib/frame-menu/dist/v1.js
var v1_exports = {};
__export(v1_exports, {
  CONVERSATION_FORMS: () => CONVERSATION_FORMS,
  UI_NAMES: () => UI_NAMES,
  isBox: () => isBox,
  isConversationForm: () => isConversationForm,
  isFoundRef: () => isFoundRef,
  isHostMessage: () => isHostMessage,
  isMenuItem: () => isMenuItem,
  isPageMessage: () => isPageMessage,
  isWelcome: () => isWelcome,
  pictureRefused: () => pictureRefused
});
function pictureRefused(src, origin, plugin) {
  let url;
  try {
    url = new URL(src);
  } catch {
    return `picture is given its full address, not "${src}".`;
  }
  if (url.protocol === "https:")
    return void 0;
  if (url.protocol === "data:")
    return /^data:image\//i.test(src) ? void 0 : "picture is given a data: address only of a picture: data:image/\u2026.";
  const own = (at) => url.pathname.startsWith(`${at}/`);
  if (url.origin === origin && (own(plugin.api) || own(plugin.base)))
    return void 0;
  if (url.protocol === "blob:")
    return "picture can't be given a blob: address: ws's page can't read it. Give it as data: instead.";
  return `picture is given an address on the web (https:), the picture itself (data:image/\u2026), or the plugin's own on ws's (under ${plugin.api}/ or ${plugin.base}/), not ${url.href}.`;
}
var UI_NAMES = ["ref-card", "thing-promote", "thing-save", "picture"];
var CONVERSATION_FORMS = ["card", "docked", "popover"];
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
  return isRecord(m) && m.type === "ws:welcome" && m.version === 1 && isString(m.ws) && isPlugin(m.plugin) && isString(m.path) && isTheme(m.theme) && isKeys(m.keys) && (m.refs === void 0 || typeof m.refs === "boolean") && (m.uis === void 0 || isStrings(m.uis)) && (m.conversations === void 0 || typeof m.conversations === "boolean") && (m.menus === void 0 || typeof m.menus === "boolean");
}
var isIndex = (v) => Number.isInteger(v) && v >= 0;
var isSize = (v) => Number.isFinite(v) && v >= 0;
function isBox(m) {
  return isRecord(m) && Number.isFinite(m.x) && Number.isFinite(m.y) && isSize(m.width) && isSize(m.height);
}
function isFoundRef(m) {
  return isRecord(m) && isIndex(m.from) && isIndex(m.to) && m.to > m.from && isString(m.ref) && isString(m.kind) && isString(m.href) && (m.shape === void 0 || isString(m.shape)) && (m.accent === void 0 || isString(m.accent));
}
var isConversationForm = (v) => CONVERSATION_FORMS.includes(v);
var optional = (v, is) => v === void 0 || is(v);
function isHolding(m) {
  return isRecord(m) && isString(m.label) && optional(m.names, isStrings) && optional(m.text, isString);
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
    case "ws:ui-open":
      return isIndex(m.id) && isString(m.ui) && m.ui !== "" && optional(m.box, isBox) && optional(m.with, isRecord);
    case "ws:ui-close":
      return isIndex(m.id);
    case "ws:here":
      return m.name === void 0 || isString(m.name) && m.name !== "";
    case "ws:menu":
      return isIndex(m.id) && typeof m.x === "number" && typeof m.y === "number" && optional(m.ref, isString) && optional(m.items, isMenuItems);
    case "ws:chrome-layout":
      return m.mode === "expanded" || m.mode === "standard";
    case "ws:conversation":
      return typeof m.ask === "boolean" && Array.isArray(m.allow) && m.allow.every(isConversationForm) && optional(m.placeholder, isString) && optional(m.holding, isHolding) && optional(m.context, isString) && optional(m.suggestions, isStrings);
    case "ws:conversation-open":
      return optional(m.message, isString) && optional(m.context, isString) && optional(m.as, isConversationForm);
    case "ws:conversation-close":
      return true;
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
    case "ws:ui-closed":
      return isIndex(m.id) && optional(m.error, isString);
    case "ws:conversation-state":
      return typeof m.open === "boolean" && optional(m.as, isConversationForm) && typeof m.working === "boolean";
    case "ws:menu-closed":
      return isIndex(m.id) && optional(m.item, isString);
    default:
      return false;
  }
}
var isBool = (v) => typeof v === "boolean";
var isMenuItems = (v) => Array.isArray(v) && v.every(isMenuItem);
function isMenuItem(m) {
  return isRecord(m) && isString(m.id) && m.id !== "" && isString(m.label) && m.label !== "" && optional(m.icon, isString) && optional(m.danger, isBool) && optional(m.disabled, isBool);
}

// ../../../../../../ui-lib/frame-menu/dist/protocol.js
var PROTOCOL_VERSIONS = [1];
function isRefuse(m) {
  return isRecord(m) && m.type === "ws:refuse" && isVersions(m.versions) && isString(m.message);
}

// ../../../../../../ui-lib/frame-menu/dist/version.js
var VERSION = "0.0.0";

// ../../../../../../ui-lib/frame-menu/dist/client.js
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
var UiError = class extends Error {
  /** The UI that was asked for. */
  ui;
  constructor(message, ui) {
    super(message);
    this.name = "UiError";
    this.ui = ui;
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
  const conversed = /* @__PURE__ */ new Set();
  const marks = /* @__PURE__ */ new Set();
  const dismissals = /* @__PURE__ */ new Set();
  const asked = /* @__PURE__ */ new Map();
  let asking = [];
  let askings = 0;
  let closed = false;
  const send = (m) => port.postMessage(m);
  const here = [];
  let told;
  const tellHere = () => {
    const name = here.at(-1)?.name;
    if (name === told || closed)
      return;
    told = name;
    send(name === void 0 ? { type: "ws:here" } : { type: "ws:here", name });
  };
  let layout = "standard";
  const talks = !!welcome.conversations;
  let settings = { ask: false, allow: [...v1_exports.CONVERSATION_FORMS] };
  let toldTalk = "";
  let status = { open: false, working: false };
  const tellTalk = () => {
    const m = { type: "ws:conversation", ...settings };
    const said = JSON.stringify(m);
    if (said === toldTalk || closed || !talks)
      return;
    toldTalk = said;
    send(m);
  };
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
  const uis = /* @__PURE__ */ new Map();
  let uiAskings = 0;
  const hasUi = new Set(welcome.uis ?? []);
  const openUi = (name, options2) => {
    if (closed)
      return Object.assign(Promise.resolve(void 0), { close() {
      } });
    if (!hasUi.has(name))
      return Object.assign(Promise.reject(new UiError(`This ws has no UI called ${name}. Update ws.`, name)), { close() {
      } });
    const id = ++uiAskings;
    const result = new Promise((resolve, reject) => uis.set(id, { ui: name, done: (r) => resolve(r), failed: reject }));
    const box = boxOf(options2.target);
    send({ type: "ws:ui-open", id, ui: name, ...box ? { box } : {}, with: { ...options2.with } });
    return Object.assign(result, {
      close() {
        const asked2 = uis.get(id);
        if (!asked2)
          return;
        uis.delete(id);
        send({ type: "ws:ui-close", id });
        asked2.done(void 0);
      }
    });
  };
  const refCard = (target, ref) => {
    if (!ref)
      throw new TypeError(`ui.refCard(target, ""): a ref is ws's word for it, as findRefs gave it.`);
    const card = openUi("ref-card", { target, with: { ref } });
    card.catch(() => {
    });
    return () => card.close();
  };
  const aboutThing = (how, thing) => {
    if (!thing?.kind || !thing.ref)
      throw new TypeError(`ui.${how}(target, thing): a thing is its kind and the plugin's name for it, { kind: "sketch", ref: "sketch-12" }.`);
    return { kind: thing.kind, ref: thing.ref, ...thing.title ? { title: thing.title } : {} };
  };
  const promoteThing = (target, thing) => openUi("thing-promote", { target, with: aboutThing("promoteThing", thing) });
  const saveThing = (target, thing) => openUi("thing-save", { target, with: aboutThing("saveThing", thing) });
  const viewPicture = (src, options2 = {}) => {
    let full = src;
    try {
      full = new URL(src, doc?.defaultView?.location.href).href;
    } catch {
    }
    const page = doc?.defaultView?.location.origin;
    const why = v1_exports.pictureRefused(full, page && page !== "null" ? page : "", welcome.plugin);
    if (why)
      throw new TypeError(`ui.viewPicture(src): ${why}`);
    return openUi("picture", { target: options2.target, with: { src: full, ...options2.title ? { title: options2.title } : {} } });
  };
  const viewClicked = (el) => {
    if (!hasUi.has("picture"))
      return false;
    const img = el.tagName === "IMG" ? el : el.querySelector("img");
    const src = el.getAttribute("data-ws-view-src") || img && (img.currentSrc || img.getAttribute("src"));
    if (!img || !src)
      return false;
    const title = el.getAttribute("data-ws-view") || img.getAttribute("alt") || img.getAttribute("title") || void 0;
    try {
      viewPicture(src, { title, target: img }).catch(() => {
      });
    } catch {
      return false;
    }
    return true;
  };
  const hasMenus = welcome.menus === true;
  const menus = /* @__PURE__ */ new Map();
  let menuAskings = 0;
  const providers = /* @__PURE__ */ new Set();
  const menu = (at2, options2 = {}) => {
    if (closed || !hasMenus)
      return Promise.resolve(false);
    const entries = options2.entries ?? [];
    for (const e of entries) {
      if (!e?.label || typeof e.run !== "function")
        throw new TypeError('ui.menu(at, { entries }): an entry is its words and what choosing it does, { label: "Rename", run() {\u2026} }.');
    }
    let x;
    let y;
    if ("clientX" in at2) {
      at2.preventDefault();
      x = at2.clientX;
      y = at2.clientY;
    } else
      ({ x, y } = at2);
    const items = entries.map((e, i) => ({
      id: String(i),
      label: e.label,
      ...e.icon ? { icon: e.icon } : {},
      ...e.danger ? { danger: true } : {},
      ...e.disabled ? { disabled: true } : {}
    }));
    const id = ++menuAskings;
    return new Promise((resolve) => {
      menus.set(id, (item) => {
        if (item !== void 0)
          entries[Number(item)]?.run();
        resolve(true);
      });
      send({ type: "ws:menu", id, x, y, ...options2.ref ? { ref: options2.ref } : {}, ...items.length ? { items } : {} });
    });
  };
  const onMenu = (provide) => {
    providers.add(provide);
    return () => {
      providers.delete(provide);
    };
  };
  const menuFor = (e) => {
    if (!hasMenus || e.defaultPrevented || !doc)
      return;
    const target = e.target;
    if (!target?.closest || isTypedIn(e.target))
      return;
    if (doc.getSelection?.()?.toString())
      return;
    const ref = target.closest("[data-ws-ref]")?.getAttribute("data-ws-ref") || void 0;
    const entries = [...providers].flatMap((p) => p(target, ref) ?? []);
    if (!ref && !entries.length)
      return;
    void menu(e, { ref, entries });
  };
  if (doc)
    applyTheme(doc, theme);
  const wired = doc ? wire(doc, welcome, send, open, refCard, viewClicked, menuFor) : void 0;
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
        wired?.left();
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
      case "ws:ui-closed": {
        const asked2 = uis.get(m.id);
        uis.delete(m.id);
        if (m.error !== void 0)
          asked2?.failed(new UiError(m.error, asked2.ui));
        else
          asked2?.done(m.result);
        break;
      }
      case "ws:conversation-state":
        status = m.as === void 0 ? { open: m.open, working: m.working } : { open: m.open, as: m.as, working: m.working };
        for (const fn of [...conversed])
          fn(status);
        break;
      case "ws:menu-closed": {
        const done = menus.get(m.id);
        menus.delete(m.id);
        done?.(m.item);
        break;
      }
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
    pushHere(name) {
      if (!name)
        throw new TypeError('pushHere(""): a name says what the page shows.');
      const entry = { name };
      here.push(entry);
      tellHere();
      let gone = false;
      return () => {
        if (gone)
          return;
        gone = true;
        here.splice(here.indexOf(entry), 1);
        tellHere();
      };
    },
    ui: { has: (name) => hasUi.has(name), open: openUi, refCard, promoteThing, saveThing, viewPicture, menu, onMenu },
    setChromeLayout(mode) {
      if (mode !== "expanded" && mode !== "standard")
        throw new TypeError(`setChromeLayout("${String(mode)}"): the layout is "expanded" or "standard".`);
      if (mode === layout || closed)
        return;
      layout = mode;
      send({ type: "ws:chrome-layout", mode });
    },
    conversation: {
      available: talks,
      get status() {
        return status;
      },
      set(say) {
        const next = { ...settings };
        for (const [key, value] of Object.entries(say)) {
          if (!["ask", "allow", "placeholder", "holding", "context", "suggestions"].includes(key))
            continue;
          if (value === void 0)
            delete next[key];
          else
            next[key] = Array.isArray(value) ? [...value] : value;
        }
        if (next.ask === void 0)
          next.ask = false;
        if (next.allow === void 0)
          next.allow = [...v1_exports.CONVERSATION_FORMS];
        const allow = next.allow;
        const bad = allow.find((f) => !v1_exports.isConversationForm(f));
        if (bad !== void 0)
          throw new TypeError(`conversation.set({ allow }): "${String(bad)}" isn't a form: they're ${v1_exports.CONVERSATION_FORMS.join(", ")}.`);
        const holding = next.holding;
        if (holding && !holding.label)
          throw new TypeError("conversation.set({ holding }): a holding has a label, for its chip.");
        settings = next;
        tellTalk();
      },
      open(options2 = {}) {
        if (options2.as !== void 0 && !v1_exports.isConversationForm(options2.as))
          throw new TypeError(`conversation.open({ as: "${String(options2.as)}" }): the forms are ${v1_exports.CONVERSATION_FORMS.join(", ")}.`);
        if (!talks || closed)
          return;
        tellTalk();
        const m = { type: "ws:conversation-open" };
        if (options2.message?.trim())
          m.message = options2.message;
        if (options2.context?.trim())
          m.context = options2.context;
        if (options2.as)
          m.as = options2.as;
        send(m);
      },
      close() {
        if (talks && !closed)
          send({ type: "ws:conversation-close" });
      },
      onChange: (fn) => on(conversed, fn)
    },
    setTitle(title) {
      if (doc)
        doc.title = title;
      send({ type: "ws:title", title });
    },
    open: (to) => open(to),
    close() {
      wired?.stop();
      for (const stop of [...marks])
        stop();
      marks.clear();
      for (const stop of [...dismissals])
        stop();
      here.length = 0;
      closed = true;
      for (const b of asking.splice(0))
        b.found(b.texts.map(() => []));
      for (const found of [...asked.values()])
        found([]);
      asked.clear();
      for (const ui of [...uis.values()])
        ui.done(void 0);
      uis.clear();
      for (const done of [...menus.values()])
        done(void 0);
      menus.clear();
      providers.clear();
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
function wire(doc, welcome, send, open, refCard, viewClicked, menuFor) {
  const { keys } = welcome;
  let leaderAt = 0;
  let over;
  let closeCard = () => {
  };
  const tell = (el) => {
    closeCard();
    closeCard = () => {
    };
    over = el;
    const ref = el?.getAttribute("data-ws-ref");
    if (el && ref)
      closeCard = refCard(el, ref);
  };
  const hover = (el) => {
    if (el !== over)
      tell(el);
  };
  const onOver = (e) => hover(refOn(e.target));
  const onOut = (e) => {
    if (!e.relatedTarget)
      hover(void 0);
  };
  const onFocus = (e) => {
    const el = refOn(e.target);
    if (el && focusShown(el))
      hover(el);
  };
  const onBlur = (e) => {
    if (over && refOn(e.target) === over)
      hover(void 0);
  };
  const left = () => hover(void 0);
  const onScroll = () => {
    if (over && doc.activeElement === over)
      tell(over);
    else
      left();
  };
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
    const viewable = target?.closest?.("[data-ws-view]");
    if (viewable && viewClicked(viewable)) {
      e.preventDefault();
      return;
    }
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
  doc.addEventListener("mouseover", onOver);
  doc.addEventListener("mouseout", onOut);
  doc.addEventListener("focusin", onFocus);
  doc.addEventListener("focusout", onBlur);
  doc.addEventListener("scroll", onScroll, true);
  doc.addEventListener("pointerdown", left, true);
  doc.addEventListener("contextmenu", menuFor);
  return {
    stop() {
      doc.removeEventListener("keydown", onKey);
      doc.removeEventListener("click", onClick);
      doc.removeEventListener("mouseover", onOver);
      doc.removeEventListener("mouseout", onOut);
      doc.removeEventListener("focusin", onFocus);
      doc.removeEventListener("focusout", onBlur);
      doc.removeEventListener("scroll", onScroll, true);
      doc.removeEventListener("pointerdown", left, true);
      doc.removeEventListener("contextmenu", menuFor);
    },
    left
  };
}
function refOn(target) {
  const el = target?.closest?.(".ws-ref[data-ws-ref]");
  return el && el.getAttribute("data-ws-ref") ? el : void 0;
}
function boxOf(target) {
  if (!target)
    return void 0;
  const { x, y, width, height } = "getBoundingClientRect" in target ? target.getBoundingClientRect() : target;
  return { x, y, width, height };
}
function focusShown(el) {
  try {
    return el.matches(":focus-visible");
  } catch {
    return true;
  }
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
  const counts = { changed: 0, themes: 0, navigated: 0, refs: 0, outside: 0, polished: 0 };
  let talk = "not said";
  let holding = false;
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
  const peeks = /* @__PURE__ */ new Set();
  function peek(t) {
    const panel = h(
      "aside",
      { className: "peek", data: { fixture: "peek" } },
      h("p", { className: "dim" }, `THING-${t.id}, peeked at`),
      h("h1", {}, t.name),
      h("p", {}, t.note),
      h(
        "p",
        {},
        h("button", { type: "button", onclick: () => close() }, "Close"),
        " ",
        h("button", { type: "button", onclick: () => peek(things?.find((o) => o.id === t.id % 3 + 1) ?? t) }, `Peek THING-${t.id % 3 + 1} over it`)
      )
    );
    panel.style.setProperty("--peeked", String(peeks.size));
    document.body.append(panel);
    const off = ws.pushHere(`THING-${t.id}`);
    const close = () => {
      off();
      panel.remove();
      peeks.delete(close);
    };
    peeks.add(close);
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
      const list = things?.map(
        (t) => h("li", {}, h("a", { href: ws.href(`/things/${t.id}`) }, t.name), " ", h("span", { className: "dim" }, t.note), " ", h("button", { type: "button", data: { fixture: "peek-button" }, onclick: () => peek(t) }, "Peek"))
      );
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
        row("ws:outside received", counts.outside, "outside"),
        row("Polished, from ws's menu", counts.polished, "polished")
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
      h(
        "p",
        {},
        h("button", { type: "button", data: { fixture: "talk-ask" }, onclick: () => ws.conversation.open({ message: "Where does it go?", context: "Asked from the fixture's button." }) }, "Ask ws something"),
        " ",
        h("button", { type: "button", data: { fixture: "talk-open" }, onclick: () => ws.conversation.open({ as: "docked" }) }, "Open the conversation"),
        " ",
        h("button", { type: "button", data: { fixture: "talk-close" }, onclick: () => ws.conversation.close() }, "Close it"),
        " ",
        h(
          "button",
          {
            type: "button",
            data: { fixture: "talk-hold" },
            onclick: () => {
              holding = !holding;
              render();
            }
          },
          holding ? "Let go of the note" : "Hold the note"
        )
      ),
      h("p", { className: "dim" }, "The conversation, on a thing's page: ", h("span", { data: { fixture: "talk" } }, talk)),
      pictures(),
      h("p", { id: "notes", className: "dim" }, "Notes: back after a filter skips it, as it replaced the entry it was on.")
    );
  }
  function pictures() {
    const src = ws.api("/api/shelf.svg");
    const thumb = h("img", { src, alt: "The shelf", className: "thumb", data: { wsView: "", fixture: "picture" } });
    const said = h("span", { className: "dim", data: { fixture: "picture-said" } }, ws.ui.has("picture") ? "" : "This ws has no viewer.");
    const ask = (to, title) => async () => {
      said.textContent = "";
      try {
        await ws.ui.viewPicture(to, { title });
        said.textContent = "Closed.";
      } catch (err) {
        said.textContent = err instanceof Error ? err.message : String(err);
      }
    };
    const dot = "data:image/svg+xml;utf8," + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><circle cx="32" cy="32" r="28" fill="#c0563f"/></svg>');
    return h(
      "p",
      { className: "pictures" },
      h("a", { href: src, target: "_blank" }, thumb),
      " ",
      h("button", { type: "button", data: { fixture: "picture-code" }, onclick: ask(dot, "A dot, from code") }, "viewPicture from code"),
      " ",
      h("button", { type: "button", data: { fixture: "picture-refused" }, onclick: ask("/api/workspace") }, "One of ws's own (refused)"),
      " ",
      said
    );
  }
  let unmark = () => {
  };
  function converse() {
    if (!ws.conversation.available) return;
    const [path, query = ""] = ws.path.split("?");
    const one = /^\/things\/(?:THING-)?(\d+)$/.exec(path);
    if (!one) return ws.conversation.set({ ask: false, allow: [] });
    ws.conversation.set({
      ask: !new URLSearchParams(query).has("nobar"),
      allow: ["card", "docked", "popover"],
      placeholder: `Ask about THING-${one[1]}\u2026`,
      suggestions: ["Where does it go?", "Put it back on the shelf"],
      context: `The fixture is showing THING-${one[1]}.`,
      ...holding ? { holding: { label: "its note", text: things?.find((t) => t.id === Number(one[1]))?.note ?? "" } } : {}
    });
  }
  function render() {
    const { title, body } = screen();
    ws.setTitle(title);
    converse();
    closeMenu();
    unmark();
    const words = names();
    ws.root.replaceChildren(h("main", {}, h("section", {}, ...body, words), status()));
    unmark = ws.markRefs(words);
  }
  ws.ui.onMenu((target, ref) => {
    const name = ref ? target.closest(".ws-ref")?.textContent?.trim() : void 0;
    return name?.startsWith("THING-") ? [
      {
        label: `Polish ${name}`,
        icon: "sparkle",
        run: () => {
          counts.polished++;
          render();
        }
      }
    ] : void 0;
  });
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
  ws.conversation.onChange((state) => {
    talk = `${state.open ? `open, ${state.as}` : "closed"}${state.working ? ", working" : ""}`;
    const cell = ws.root.querySelector('[data-fixture="talk"]');
    if (cell) cell.textContent = talk;
  });
  render();
  await load();
}
void main();
