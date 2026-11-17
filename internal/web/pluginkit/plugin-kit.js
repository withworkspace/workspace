// The page's side of a plugin's view in ws's web UI: ws serves this at
// /plugin-kit.js, and a plugin's page includes it in its <head>. It gives
// the page ws's theme, keeps ws's address and title in step with the
// page's, passes on the keys ws owns, and reloads the page when the plugin
// says what it shows is stale. See PROTOCOL.md in the plugin framework,
// github.com/withworkspace/plugin.
(function () {
  "use strict";
  var framed = window.parent !== window;
  var root = document.documentElement;
  var base = (document.querySelector("base") || {}).href || "";
  var prefix = base ? new URL(base).pathname.replace(/\/$/, "") : "";
  var onChanged = [];
  var onNavigate = [];
  var onReady = [];
  var ready = false;
  var gAt = 0;

  function applyTheme(t) {
    if (!t) return;
    if (t.scheme) {
      root.dataset.theme = t.scheme;
      root.style.colorScheme = t.scheme;
    }
    for (var name in t.vars || {}) root.style.setProperty(name, t.vars[name]);
  }

  // Before the first paint, the theme is asked of ws's page directly (the
  // frame is same-origin), so the page never flashes its defaults. Which
  // tokens there are is ws's to say, not this script's.
  function themeFromParent() {
    try {
      return window.parent.wsPluginTheme();
    } catch (e) {
      return null; // not in ws, or one from before it said: ws:hello will
    }
  }

  function post(type, fields) {
    if (!framed) return;
    var m = { type: type };
    for (var k in fields || {}) m[k] = fields[k];
    window.parent.postMessage(m, window.location.origin);
  }

  // The page's path as ws knows it: without the base it's served under.
  function here() {
    var p = window.location.pathname;
    if (prefix && p.indexOf(prefix) === 0) p = p.slice(prefix.length);
    return (p || "/") + window.location.search;
  }

  var ws = {
    // ready(fn) runs fn once ws has answered, with the page's path.
    ready: function (fn) {
      if (ready) fn({ path: here() });
      else onReady.push(fn);
    },
    // path(p) tells ws the page moved to p without loading (its own routing).
    path: function (p) { post("ws:path", { path: p }); },
    // onNavigate(fn) is for a page that routes itself: when ws's address
    // moves to somewhere else in the plugin (the sidebar, back and
    // forward, a link in ws), fn is given the path, and the page isn't
    // loaded again. Without it, ws loads the page at the new path.
    onNavigate: function (fn) {
      onNavigate.push(fn);
      post("ws:routing", {});
    },
    // title(t) is the page's title, for the tab.
    title: function (t) { document.title = t; post("ws:title", { title: t }); },
    // open(p) goes to a page of ws's own, outside the plugin:
    // /missions/token-cache. A link does the same with data-ws:
    // <a data-ws="/missions/token-cache">.
    open: function (p) {
      if (framed) post("ws:open", { path: p });
      else window.location.href = p;
    },
    // onChanged(fn) runs fn when the plugin says what's shown is stale.
    // With none set, the page reloads.
    onChanged: function (fn) { onChanged.push(fn); },
    // url(p) is a path in the plugin's view as an address: /browse/X.
    url: function (p) { return prefix + (p.charAt(0) === "/" ? p : "/" + p); },
    framed: framed,
  };
  window.ws = ws;

  if (framed) applyTheme(themeFromParent());

  window.addEventListener("message", function (e) {
    if (e.origin !== window.location.origin || !e.data || typeof e.data.type !== "string") return;
    switch (e.data.type) {
      case "ws:hello":
        applyTheme(e.data);
        if (!ready) {
          ready = true;
          onReady.splice(0).forEach(function (fn) { fn({ path: here() }); });
        }
        break;
      case "ws:theme":
        applyTheme(e.data);
        break;
      case "ws:navigate":
        if (typeof e.data.path !== "string") break;
        if (onNavigate.length) onNavigate.forEach(function (fn) { fn(e.data.path); });
        else window.location.replace(ws.url(e.data.path)); // it asked, and has since stopped listening
        break;
      case "ws:changed":
        if (onChanged.length) onChanged.forEach(function (fn) { fn(); });
        else if (!document.querySelector("form[data-dirty]")) window.location.reload();
        break;
    }
  });

  // The keys ws owns work in the frame as they do outside it: ⌘K and
  // Ctrl-K (the palette) always, and its bare keys when nothing's being
  // typed in.
  window.addEventListener("keydown", function (e) {
    if (!framed) return;
    var t = e.target;
    var typing = t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName));
    var chord = (e.metaKey || e.ctrlKey) && (e.key === "k" || e.key === "p");
    // g then a letter goes somewhere in ws: both are ws's.
    var after = !typing && Date.now() - gAt < 900 && /^[a-z]$/.test(e.key);
    var bare = !typing && !e.metaKey && !e.ctrlKey && !e.altKey && (e.key === "?" || e.key === "g" || after);
    if (bare) gAt = e.key === "g" ? Date.now() : 0;
    if (!chord && !bare) return;
    e.preventDefault();
    post("ws:key", { key: e.key, meta: e.metaKey, ctrl: e.ctrlKey, shift: e.shiftKey, alt: e.altKey });
  });

  // A form that's been typed in isn't reloaded from under its writer.
  document.addEventListener("input", function (e) {
    var f = e.target && e.target.form;
    if (f) f.setAttribute("data-dirty", "");
  });

  // A link to one of ws's own pages, <a data-ws="/missions/x">, goes there
  // in ws, not in the frame.
  document.addEventListener("click", function (e) {
    var a = e.target && e.target.closest && e.target.closest("a[data-ws]");
    if (!a || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    ws.open(a.getAttribute("data-ws"));
  });

  function announce() {
    post("ws:ready", { path: here(), title: document.title });
    if (!framed) {
      ready = true;
      onReady.splice(0).forEach(function (fn) { fn({ path: here() }); });
    }
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", announce);
  else announce();
})();
