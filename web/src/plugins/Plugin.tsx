import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { type Plugin } from "../api";
import { type IconName, icons } from "../ui/Icon";
import { Empty, useTitle } from "../ui/kit";

// A plugin's view: its own pages, served by the plugin (/plugins/<name>/…)
// and shown here in a frame, at /x/<name>/…. The frame is this origin's
// own, not sandboxed: a plugin is a program the workspace's owner
// installed. The page and ws talk by postMessage (PROTOCOL.md in
// workspace-kit): the page says where it is and what it's called, and ws
// keeps its address and title in step; ws gives it the theme, and says
// when the plugin's said what it shows is stale.

// pluginIcon is the icon a plugin asked for, if ws has it.
export function pluginIcon(name?: string): IconName {
  return name && name in icons ? (name as IconName) : "box";
}

// The design tokens a plugin's page is given, as --ws-<name>: ws's own
// (styles.css), under the names a plugin's page knows them by. This is the
// one list of them: /plugin-kit.js asks for it (wsPluginTheme, below), and
// /plugin-kit.css has what each is before it's given.
const tokens = [
  // Colour: the page, what's on it, and what marks it.
  "bg", "panel", "raised", "fg", "dim", "faint", "line", "line-strong", "hover", "well", "code-bg",
  "accent", "accent-soft", "accent-line", "primary", "primary-hover", "on-primary", "danger", "warn", "ok",
  // Type.
  "font", "mono", "text-xs", "text-sm", "text-md", "text-lg", "text-xl", "text-2xl",
  // Space, corners and depth.
  "space-1", "space-2", "space-3", "space-4", "space-5", "space-6", "space-8", "space-10",
  "radius", "radius-sm", "radius-lg", "shadow",
];
// ws's own name for one, where it differs.
const tokenAlias: Record<string, string> = { danger: "red", warn: "amber", ok: "green" };

// theme is ws's theme as a plugin's page takes it: light or dark, and each
// token's value now.
export function theme() {
  const root = document.documentElement;
  const style = getComputedStyle(root);
  const vars: Record<string, string> = {};
  for (const name of tokens) {
    const v = style.getPropertyValue(`--${tokenAlias[name] ?? name}`).trim();
    if (v) vars[`--ws-${name}`] = v;
  }
  const scheme = root.dataset.theme ?? (style.colorScheme.includes("dark") ? "dark" : "light");
  return { scheme, vars };
}

// A plugin's page reads the theme from here as it loads (the frame is this
// origin's), so it has it before it first paints and never flashes its
// defaults. ws:hello says the same once the page asks.
(window as unknown as { wsPluginTheme?: typeof theme }).wsPluginTheme = theme;

export function PluginPage({ plugins }: { plugins?: Plugin[] }) {
  const { name = "", "*": rest = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const frame = useRef<HTMLIFrameElement>(null);
  const plugin = plugins?.find((p) => p.name === name);
  const [title, setTitle] = useState<string>();
  useTitle(title ?? plugin?.views[0]?.title ?? name);

  // Where the address says the frame should be, and where the frame last
  // said it is. The frame is sent somewhere only when the address moved
  // without it: a link in ws, the sidebar, back and forward.
  const wanted = `/${rest}${location.search}`;
  const shown = useRef(wanted);
  // The frame starts where the address is. After that it's sent by hand:
  // its src doesn't say where it's gone to since.
  const [start] = useState(`/plugins/${name}${wanted}`);
  // A page that routes itself (ws.onNavigate in /plugin-kit.js) is told the
  // path, and isn't loaded again. It says so each time it loads.
  const routing = useRef(false);
  useEffect(() => {
    if (shown.current === wanted) return;
    shown.current = wanted;
    const inner = frame.current?.contentWindow;
    if (routing.current) inner?.postMessage({ type: "ws:navigate", path: wanted }, window.location.origin);
    else inner?.location.replace(`/plugins/${name}${wanted}`);
  }, [name, wanted]);

  useEffect(() => {
    const post = (m: object) => frame.current?.contentWindow?.postMessage(m, window.location.origin);
    const moved = (path: unknown) => {
      if (typeof path !== "string" || !path.startsWith("/")) return;
      if (shown.current === path) return;
      shown.current = path;
      navigate(`/x/${name}${path}`, { replace: true });
    };
    const onMessage = (e: MessageEvent) => {
      if (e.origin !== window.location.origin || e.source !== frame.current?.contentWindow) return;
      const m = e.data as { type?: string; path?: unknown; title?: unknown; key?: string; meta?: boolean; ctrl?: boolean; shift?: boolean; alt?: boolean };
      switch (m?.type) {
        case "ws:ready":
          routing.current = false; // a page newly loaded: it says again if it routes itself
          post({ type: "ws:hello", ...theme() });
          moved(m.path);
          if (typeof m.title === "string" && m.title) setTitle(m.title);
          break;
        case "ws:routing":
          routing.current = true;
          break;
        case "ws:path":
          moved(m.path);
          break;
        case "ws:title":
          if (typeof m.title === "string") setTitle(m.title);
          break;
        case "ws:open":
          if (typeof m.path === "string" && m.path.startsWith("/") && !m.path.startsWith("//")) navigate(m.path);
          break;
        case "ws:key":
          // A key ws owns, pressed in the frame: as if it were pressed here.
          if (typeof m.key === "string")
            window.dispatchEvent(new KeyboardEvent("keydown", { key: m.key, metaKey: !!m.meta, ctrlKey: !!m.ctrl, shiftKey: !!m.shift, altKey: !!m.alt, bubbles: true, cancelable: true }));
          break;
      }
    };
    window.addEventListener("message", onMessage);
    // The theme and the accent are on <html>: when either changes, the
    // frame's told.
    const watch = new MutationObserver(() => post({ type: "ws:theme", ...theme() }));
    watch.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "style"] });
    return () => {
      window.removeEventListener("message", onMessage);
      watch.disconnect();
    };
  }, [name, navigate]);

  // The plugin said what its view shows is stale: the page is told, and
  // reloads itself unless it does something of its own.
  const changed = plugin?.changed;
  const seen = useRef(changed);
  useEffect(() => {
    if (changed === undefined || seen.current === changed) return;
    const first = seen.current === undefined;
    seen.current = changed;
    if (!first) frame.current?.contentWindow?.postMessage({ type: "ws:changed" }, window.location.origin);
  }, [changed]);

  // It was started again (built afresh, its config changed), or came to be
  // set up: what the frame shows is from before, so it's loaded again.
  const state = plugin?.state;
  const starts = plugin?.starts;
  const was = useRef({ state, starts });
  useEffect(() => {
    const before = was.current;
    was.current = { state, starts };
    if (before.state === undefined) return; // the first it's heard of
    const restarted = starts !== undefined && before.starts !== starts && starts > 0;
    const recovered = before.state !== state && (before.state === "setup" || before.state === "failed");
    if (restarted || recovered) frame.current?.contentWindow?.location.reload();
  }, [state, starts]);

  if (plugins && !plugin) {
    return (
      <Empty icon="box" title={`There's no plugin called ${name}`}>
        <code>ws plugin list</code> says which this workspace has.
      </Empty>
    );
  }
  return (
    <div className="plugin-view">
      <iframe ref={frame} src={start} title={plugin?.views[0]?.title ?? name} />
    </div>
  );
}
