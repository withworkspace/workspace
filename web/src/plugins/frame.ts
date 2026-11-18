import { chooseVersion, type Connect, isConnect, type Refuse } from "@withworkspace/ui-lib/protocol";
import type { Theme } from "./theme";
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
  if (!page.length) return `${plugin} lists no frame protocol version, and this ws speaks ${host.join(", ")}. Update the plugin.`;
  const ahead = Math.max(...page) > Math.max(...host);
  return `${plugin} speaks frame protocol ${page.join(", ")}, and this ws speaks ${host.join(", ")}. Update ${ahead ? "ws" : "the plugin"}.`;
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
  // Stops listening for the frame, and hangs up on the page in it.
  stop(): void;
}

// listen answers each ws:connect from the frame's window, until it's
// stopped. Only the frame's own window is answered, from this origin, and
// only with a port: anything else is somebody else's message. Each load of
// the frame connects again, and the page before is hung up on. refused is
// told what ws said to a page it couldn't meet, and undefined once one
// connects. Call it before the frame is made (a layout effect), so the
// frame's first ws:connect can't come before ws is listening.
export function listen(frame: () => Window | null | undefined, host: Host, refused: (message: string | undefined) => void): Frame {
  let session: Session | undefined;
  const onMessage = (e: MessageEvent) => {
    const source = frame();
    if (!source || e.source !== source || e.origin !== window.location.origin) return;
    const port = e.ports[0];
    if (!port || !isConnect(e.data)) return;
    session?.close();
    session = undefined;
    const a = answer(e.data, port, host);
    if ("refused" in a) {
      // The page has it too, as an error: this is for whoever's looking at
      // ws's console to see who was asking.
      console.warn(`${a.refused} (${e.data.client ?? "a page that didn't say what it is"})`);
      refused(a.refused);
      return;
    }
    session = a.session;
    refused(undefined);
  };
  window.addEventListener("message", onMessage);
  return {
    navigate: (path) => session?.navigate(path),
    theme: () => session?.theme(),
    changed: () => session?.changed(),
    stop() {
      window.removeEventListener("message", onMessage);
      session?.close();
      session = undefined;
    },
  };
}
