import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { type Plugin } from "../api";
import { createHost } from "../conversation/host";
import { PageConversation } from "../conversation/PageConversation";
import { refAt, thingOpenAt } from "../refs/refs";
import { type IconName, icons } from "../ui/Icon";
import { refsFound } from "../refs/found";
import { useRefs, useRefsWorkspace } from "../refs/known";
import { Empty, useTitle } from "../ui/kit";
import { setFrameDim } from "./dim";
import { setFrameHere, useFrameHere } from "./here";
import { setFrameLayout } from "./layout";
import { type Frame, listen } from "./frame";
import { frameBase, viewBase } from "./paths";
import { onThemeChange, theme } from "./theme";
import { frameUis } from "./ui";
import { FrameUiLayer } from "./Uis";

// A plugin's view: the plugin's app, in a frame, at /x/<name>/…. The frame
// is a page ws serves (the shell, /plugin-frame/<name>/…: ws's tokens, an
// empty #root and the plugin's bundle), at the same path as the address
// here, so a link to somewhere deep in the app loads it there. It's this
// origin's own, not sandboxed: a plugin is a program the workspace's owner
// installed. The app and ws talk over a port the app hands over as it
// connects (the frame protocol: PROTOCOL.md in github.com/withworkspace/ui-lib).
// ws keeps the history and the frame keeps none: every link clicked in the
// app, and every move it makes, is the app asking ws to go there, and ws
// says where the app is whenever its address moves within the plugin. The
// app says what it's called; ws says when the theme changed, and when the
// plugin said what the app shows is stale. The names in the app's words
// (a file, a mission, a plugin's thing) are ws's to find: the app sends
// the words it shows and draws the refs ws finds in them, and ws says when
// what there is to name has changed. ws has UIs of its own the app can
// ask for, which ws defines and draws over the frame, by where in the
// app's page each was asked from: a ref's card, by its words (ui.ts).
// And the frame's edge is no edge to
// whoever's looking: a press in ws is said to the app, which never sees
// one, so its menus close as they would for a press in the app; and
// while the app has a backdrop up, ws dims what's beside the frame. The
// conversation about the page's thing is ws's, drawn round the frame
// (conversation/PageConversation.tsx): the app says how it may be had
// (ws:conversation), as ws's own pages do.

// pluginIcon is the icon a plugin asked for, if ws has it.
export function pluginIcon(name?: string): IconName {
  return name && name in icons ? (name as IconName) : "box";
}

// A plugin's page is its own frame and its own connection: another
// plugin's is another of each.
export function PluginPage({ plugins, ws }: { plugins?: Plugin[]; ws?: string }) {
  const { name = "" } = useParams();
  return <PluginFrame key={name} name={name} plugins={plugins} ws={ws} />;
}

function PluginFrame({ name, plugins, ws }: { name: string; plugins?: Plugin[]; ws?: string }) {
  const location = useLocation();
  const navigate = useNavigate();
  const iframe = useRef<HTMLIFrameElement>(null);
  const plugin = plugins?.find((p) => p.name === name);
  const [title, setTitle] = useState<string>();
  useTitle(title ?? plugin?.views[0]?.title ?? name);
  // What ws said to an app it has no frame protocol version in common
  // with: shown above the frame, as whoever's looking at an empty frame
  // won't open the console.
  const [refused, setRefused] = useState<string>();

  // Where the address says the app should be, and where the app was last
  // told it is. Whatever moved the address (the app, a link in ws, the
  // sidebar, back and forward), the app is told.
  const wanted = (location.pathname.slice(viewBase(name).length) || "/") + location.search;
  const shown = useRef(wanted);
  // The frame starts where the address is. After that the app routes
  // itself: the frame's src doesn't say where it's gone to since.
  const [start] = useState(`${frameBase(name)}${wanted}`);
  const frame = useRef<Frame>(undefined);
  // The conversation about what the page is of: ws's own, which the app
  // drives over the frame, remembering the person's choice of form for
  // the plugin.
  const [conversation] = useState(() => createHost(name));
  // The UIs of ws's own the app has asked for (a ref's card, by its
  // words), drawn over the frame; what comes of one ws closes, the app
  // is told.
  const [uis] = useState(() => frameUis((id, result) => frame.current?.uiClosed(id, result)));

  // What there is to name, and the workspace: what the refs in the app's
  // words are found by, and drawn from.
  const refs = useRefs();
  const workspace = useRefsWorkspace();

  // What the connection's given is read when it's asked for, so the
  // listening below is set up once, and never torn down for a new render.
  const now = useRef({ ws, navigate, refs, workspace });
  now.current = { ws, navigate, refs, workspace };

  // ws listens before the frame is made, so the app's first ws:connect
  // can't come before there's anyone to hear it.
  useLayoutEffect(() => {
    const f = listen(
      () => iframe.current?.contentWindow,
      {
        plugin: name,
        get ws() {
          return now.current.ws || "unknown";
        },
        path: () => shown.current,
        theme,
        // An empty title is the view's own.
        titled: (t) => setTitle(t || undefined),
        // What a go does is ws's to decide: today, any address of ws's
        // own, the plugin's pages among them. Going to where ws already
        // is adds no history, as a link to the page you're on doesn't.
        go(path, replace) {
          if (!path.startsWith("/") || path.startsWith("//")) return;
          const here = window.location.pathname + window.location.search + window.location.hash;
          now.current.navigate(path, { replace: replace || path === here });
        },
        // A key ws owns, pressed in the frame: as if it were pressed here.
        key(k) {
          window.dispatchEvent(new KeyboardEvent("keydown", { key: k.key, metaKey: k.meta, ctrlKey: k.ctrl, shiftKey: k.shift, altKey: k.alt, bubbles: true, cancelable: true }));
        },
        // The refs in the app's words, found as in ws's own pages. A repo
        // a text names has its files read, so its files can be found: the
        // app is told when they have been (what there is to name changed).
        refs(texts) {
          const { refs, workspace } = now.current;
          if (!refs || !workspace) return texts.map(() => []);
          refs.want(texts.join("\n"));
          return texts.map((text) => refsFound(text, { known: refs.known, repos: refs.repos, workspace }));
        },
        // The app has a backdrop up, as ws sees in its page: the shell
        // dims what's beside the frame (App.tsx) for as long as it has.
        dim: setFrameDim,
        // What the app asks ws to show of its own, over the frame.
        ui: uis,
        // What the app shows beyond its address: Find starts from it.
        here: (n) => setFrameHere(name, n),
        // How the app wants the chrome: expanded, the shell folds its
        // sidebar (App.tsx).
        chromeLayout: (mode) => setFrameLayout(name, mode),
        conversation,
      },
      setRefused,
    );
    frame.current = f;
    // The theme and the accent are on <html>: when either changes, the app
    // is given every token's value as it is now.
    const unwatch = onThemeChange(() => f.theme());
    // A press anywhere in ws's own page is one outside the frame: a press
    // in the frame goes to the frame's window and never comes here. It's
    // heard on the way down, before whatever is pressed can stop it.
    const pressed = () => f.outside();
    window.addEventListener("pointerdown", pressed, true);
    // Where the conversation is, the app hears.
    const unheard = conversation.onChange((s) => f.conversation(s));
    return () => {
      unwatch();
      unheard();
      window.removeEventListener("pointerdown", pressed, true);
      // Hanging up ends the dim, if the app had a backdrop up.
      f.stop();
      frame.current = undefined;
    };
  }, [name]);

  useEffect(() => {
    if (shown.current === wanted) return;
    shown.current = wanted;
    // What's shown on hover is of the page as it was.
    uis.unhover();
    frame.current?.navigate(wanted);
  }, [wanted]);

  // What there is to name changed (a mission began, a repo's files were
  // read, a plugin registered its names): an app that asked which of its
  // words are names is told, and asks again.
  const known = refs?.known;
  const named = useRef(known);
  useEffect(() => {
    if (named.current === known) return;
    named.current = known;
    frame.current?.refsChanged();
  }, [known]);

  // The plugin said what its view shows is stale: the app is told.
  const changed = plugin?.changed;
  const seen = useRef(changed);
  useEffect(() => {
    if (changed === undefined || seen.current === changed) return;
    const first = seen.current === undefined;
    seen.current = changed;
    if (!first) frame.current?.changed();
  }, [changed]);

  // It was started again (built afresh, its config changed), or came to be
  // set up: what the frame shows is from before, so it's loaded again,
  // where ws's address is. (The frame's own address is where the app
  // started, or wherever its router has put it since.)
  const state = plugin?.state;
  const starts = plugin?.starts;
  const was = useRef({ state, starts });
  useEffect(() => {
    const before = was.current;
    was.current = { state, starts };
    if (before.state === undefined) return; // the first it's heard of
    const restarted = starts !== undefined && before.starts !== starts && starts > 0;
    const recovered = before.state !== state && (before.state === "setup" || before.state === "failed");
    if (!restarted && !recovered) return;
    setRefused(undefined);
    setTitle(undefined);
    iframe.current?.contentWindow?.location.replace(`${frameBase(name)}${shown.current}`);
  }, [name, state, starts]);

  // What the page is of, to ask about: what the app said it shows, or
  // else what its address is, as the thing's kind said its pages are.
  const frameHere = useFrameHere();
  const said = frameHere?.plugin === name && known ? refAt(frameHere.name, known) : undefined;
  const open = said?.kind === "thing" ? said : known ? thingOpenAt(name, location.pathname.slice(viewBase(name).length) || "/", known) : undefined;
  const about = open ? { thing: open.id, plugin: name, from: plugin?.views[0]?.title ?? name } : undefined;

  if (plugins && !plugin) {
    return (
      <Empty icon="box" title={`There's no plugin called ${name}`}>
        <code>ws plugin list</code> says which this workspace has.
      </Empty>
    );
  }
  return (
    <div className="plugin-view">
      {refused && (
        <div className="banner plugin-refused" role="alert">
          {refused}
        </div>
      )}
      <PageConversation host={conversation} about={about} root={workspace?.root}>
        <iframe ref={iframe} src={start} title={plugin?.views[0]?.title ?? name} onLoad={() => frame.current?.loaded()} />
      </PageConversation>
      <FrameUiLayer uis={uis} frame={iframe} plugin={name} />
    </div>
  );
}
