import { chooseVersion, type Connect, isConnect, type Refuse, type V1 } from "@withworkspace/ui-lib/protocol";
import type { ConversationHost, Status } from "../conversation/host";
import type { FoundRef } from "../refs/found";
import { watchBackdrop } from "./backdrop";
import type { Theme } from "./theme";
import type { Asked } from "./ui";
import { v1 } from "./frame/v1";

// ws's side of the frame protocol's handshake (PROTOCOL.md in
// github.com/withworkspace/ui-lib; a page's side is @withworkspace/ui-lib).
// A plugin's app, in its frame, makes a channel and sends ws:connect to
// this window with one end of it and the versions it speaks. ws picks the
// highest both speak and hands the port to that version's module, or says
// on the port that it speaks none of them. Everything after is on the port.

// KeyPress is a key pressed in the frame that's one of ws's.
export interface KeyPress {
  key: string;
  meta: boolean;
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
}

// Host is what ws is to a plugin's page, whichever version it speaks: what
// it's told as it connects, and what ws does with what it says. A
// version's module says it as that version has it.
export interface Host {
  // The plugin's name.
  readonly plugin: string;
  // ws's own version.
  readonly ws: string;
  // Where in the plugin ws's address is now: /browse/PROJ-1?tab=links.
  path(): string;
  // ws's theme now.
  theme(): Theme;
  // The page's title; "" for the view's own.
  titled(title: string): void;
  // Go to this address of ws's: a page in the plugin (/x/jira/browse/2),
  // or one of ws's own. Every link and move in the app is one: ws keeps
  // the history, and tells the app with navigate when it's in the plugin.
  go(path: string, replace: boolean): void;
  // A key ws owns was pressed in the frame.
  key(press: KeyPress): void;
  // The refs in each of some runs of text the app shows, as ws finds them
  // in its own pages (refs/found.ts): the app draws what it's told, since
  // only ws knows what there is to name.
  refs(texts: string[]): FoundRef[][];
  // The UIs of ws's own the app asks for (ui.ts): a ref's card, by its
  // words. ws defines each and draws it over the frame, by where in the
  // app's page it was asked from.
  ui: {
    // The ones there are, by name: the app is told as it connects, so
    // what it awaits of one ws hasn't fails at once.
    readonly names: readonly string[];
    // The app asks for one. What comes back is why it can't be shown, if
    // it can't, and the app is told.
    open(asked: Asked): string | undefined;
    // The app takes one back.
    close(id: number): void;
    // The page was hung up on: all it asked for goes with it.
    forget(): void;
  };
  // The app has a backdrop up, or has taken its last one down: ws dims
  // what's beside the frame for as long as it has one. No version says
  // it: ws sees the backdrop in the page that's connected (backdrop.ts).
  // The dim is off, too, once that page is hung up on.
  dim(on: boolean): void;
  // What the app shows now that its address doesn't say: one of ws's
  // names (an issue in a panel), or none. Find starts from it. It's none,
  // too, once the page is hung up on.
  here(name: string | undefined): void;
  // How the app wants ws's chrome laid out: expanded, ws folds its
  // sidebar to a rail. It's standard again once the page is hung up on.
  chromeLayout(mode: V1.ChromeMode): void;
  // A right click in the app: ws's menu, at x, y in the frame's viewport,
  // holding the app's own items first, then what ws does to ref. What
  // came of it is said with the session's menuClosed.
  menu(asked: { id: number; x: number; y: number; ref?: string; items: V1.MenuItem[] }): void;
  // The conversation about the page: ws's own (conversation/host.ts),
  // which the page drives as ws's own pages do. What the page said of it
  // is forgotten once the page is hung up on.
  conversation: ConversationHost;
}

// Session is a page that's connected, as ws tells it things.
export interface Session {
  // The frame protocol version it's in.
  readonly version: number;
  // ws's address moved within the plugin: the app shows that path.
  navigate(path: string): void;
  // The theme changed: the host's theme, as it is now.
  theme(): void;
  // The plugin said what its view shows is stale.
  changed(): void;
  // What there is to name changed: what the app was told of the refs in
  // its words may be different now.
  refsChanged(): void;
  // A press landed in ws, outside the frame: the app never sees one.
  outside(): void;
  // ws closed one of its UIs the app had asked for: dismissed, or done,
  // with what came of it.
  uiClosed(id: number, result?: unknown): void;
  // ws's menu the app asked for was put away: with the app's own item
  // that was chosen, if one was.
  menuClosed(id: number, item?: string): void;
  // Where the conversation about the page is now.
  conversation(status: Status): void;
  // Stops listening, and closes the port.
  close(): void;
}

// Speak is one version's host: given the port of a page that speaks it, it
// welcomes the page, and listens to it.
export type Speak = (port: MessagePort, host: Host) => Session;

// The versions ws speaks, each a module of its own. A new version adds
// one and leaves the rest as they are, so a page built for an old version
// goes on working: dropping one breaks every plugin that speaks only it.
export const versions: Readonly<Record<number, Speak>> = { 1: v1 };

// refusal is what ws says to a page it has no version in common with:
// which side is behind. The page is given it (its connect() rejects with
// it), and ws shows it above the frame.
export function refusal(plugin: string, page: readonly number[], host: readonly number[]): string {
  if (!page.length) return `${plugin} lists no frame protocol version; Workspace supports ${host.join(", ")}. Update the plugin.`;
  const ahead = Math.max(...page) > Math.max(...host);
  return `${plugin} supports frame protocol ${page.join(", ")}; Workspace supports ${host.join(", ")}. Update ${ahead ? "Workspace" : "the plugin"}.`;
}

// answer is ws's answer to a ws:connect, on the port that came with it: a
// session in the highest version both speak, or a refusal, said on the
// port and returned.
export function answer(connect: Connect, port: MessagePort, host: Host, speaks: Readonly<Record<number, Speak>> = versions): { session: Session } | { refused: string } {
  const ours = Object.keys(speaks).map(Number);
  const version = chooseVersion(connect.versions, ours);
  if (version === undefined) {
    const message = refusal(host.plugin, connect.versions, ours);
    // ws:refuse holds across versions: it's said before there's one.
    port.postMessage({ type: "ws:refuse", versions: ours, message } satisfies Refuse);
    port.close();
    return { refused: message };
  }
  return { session: speaks[version](port, host) };
}

// Frame is a plugin's frame as the page that holds it speaks to it:
// through whichever page is connected now, and to nobody while none is.
export interface Frame {
  navigate(path: string): void;
  theme(): void;
  changed(): void;
  refsChanged(): void;
  outside(): void;
  uiClosed(id: number, result?: unknown): void;
  menuClosed(id: number, item?: string): void;
  conversation(status: Status): void;
  // The frame loaded a page. If it isn't the one that's connected, that
  // one is gone, and is hung up on: a page that went with a backdrop up,
  // with nothing connecting after it, doesn't leave ws dimmed.
  loaded(): void;
  // Stops listening for the frame, and hangs up on the page in it.
  stop(): void;
}

// listen answers each ws:connect from the frame's window, until it's
// stopped. Only the frame's own window is answered, from this origin, and
// only with a port: anything else is somebody else's message. Each load of
// the frame connects again, and the page before is hung up on. refused is
// told what ws said to a page it couldn't meet, and undefined once one
// connects. Call it before the frame is made (a layout effect), so the
// frame's first ws:connect can't come before ws is listening. The page
// that's connected is watched for a backdrop (watch: backdrop.ts), and the
// host told as it has one and as it hasn't.
export function listen(
  frame: () => Window | null | undefined,
  host: Host,
  refused: (message: string | undefined) => void,
  watch: (page: Document, said: (on: boolean) => void) => () => void = watchBackdrop,
): Frame {
  let session: Session | undefined;
  // The document of the page that's connected: the frame's window stays
  // the same from one load to the next, and its document doesn't.
  let page: Document | undefined;
  let unwatch = () => {};
  // A page that's gone has no backdrop up, whatever it had.
  const hangUp = () => {
    if (!session) return;
    session.close();
    session = undefined;
    page = undefined;
    unwatch();
    unwatch = () => {};
    host.dim(false);
    host.ui.forget();
    host.here(undefined);
    host.chromeLayout("standard");
    host.conversation.forget();
  };
  const onMessage = (e: MessageEvent) => {
    const source = frame();
    if (!source || e.source !== source || e.origin !== window.location.origin) return;
    const port = e.ports[0];
    if (!port || !isConnect(e.data)) return;
    hangUp();
    const a = answer(e.data, port, host);
    if ("refused" in a) {
      // The page has it too, as an error: this is for whoever's looking at
      // ws's console to see who was asking.
      console.warn(`${a.refused} (${e.data.client ?? "a page that didn't say what it is"})`);
      refused(a.refused);
      return;
    }
    session = a.session;
    page = documentOf(source);
    if (page) unwatch = watch(page, (on) => host.dim(on));
    refused(undefined);
  };
  window.addEventListener("message", onMessage);
  return {
    navigate: (path) => session?.navigate(path),
    theme: () => session?.theme(),
    changed: () => session?.changed(),
    refsChanged: () => session?.refsChanged(),
    outside: () => session?.outside(),
    uiClosed: (id, result) => session?.uiClosed(id, result),
    menuClosed: (id, item) => session?.menuClosed(id, item),
    conversation: (status) => session?.conversation(status),
    loaded() {
      const now = frame();
      if (now && documentOf(now) !== page) hangUp();
    },
    stop() {
      window.removeEventListener("message", onMessage);
      hangUp();
    },
  };
}

// The document a window shows now. The frame is this origin's own, so
// it's there to read: one that isn't has no page of a plugin's in it.
function documentOf(win: Window): Document | undefined {
  try {
    return win.document;
  } catch {
    return undefined;
  }
}
