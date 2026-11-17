import { connect, ProtocolError } from "@withworkspace/ui-lib";
import { isRefuse, TOKENS, V1 } from "@withworkspace/ui-lib/protocol";
import { afterEach, describe, expect, it, vi } from "vitest";
import { frameKeys } from "../keys";
import { answer, type Host, type KeyPress, listen, refusal, type Session, type Speak, versions } from "./frame";
import { v1 } from "./frame/v1";
import { tokenVar } from "./tokens";

// ws's side of the frame protocol, against the page's side as it's
// published: @withworkspace/ui-lib's own client connects to ws's host here,
// over a real MessageChannel, so the two are tried together as well as
// typed from one definition.

const vars = (value: string) => Object.fromEntries(TOKENS.map((t) => [tokenVar(t), value])) as V1.Vars;

// A host that keeps what the page said, with a theme a test can change.
function fakeWs(path = "/things/2?tab=links") {
  const said = { moved: [] as string[], titled: [] as string[], opened: [] as string[], keys: [] as KeyPress[] };
  let theme: V1.Theme = { scheme: "dark", vars: vars("dark") };
  const host: Host = {
    plugin: "jira",
    ws: "1.2.3",
    path: () => path,
    theme: () => theme,
    moved: (p) => void said.moved.push(p),
    titled: (t) => void said.titled.push(t),
    open: (p) => void said.opened.push(p),
    key: (k) => void said.keys.push(k),
  };
  return { host, said, setTheme: (t: V1.Theme) => void (theme = t) };
}

// The page's client, connected to ws's answer: what a plugin's app has.
async function connected(ws = fakeWs()) {
  let session: Session | undefined;
  const client = await connect({
    host: {
      postMessage(message, _origin, [port]) {
        const a = answer(message as V1.Connect, port as MessagePort, ws.host);
        if ("session" in a) session = a.session;
      },
    },
  });
  if (!session) throw new Error("ws refused the client");
  return { client, session, ...ws };
}

// A turn of the loop: a message posted on a port has arrived after it.
const tick = () => new Promise((r) => setTimeout(r, 5));

// One message a page would get on its port, from ws's answer to connect.
async function answered(connectMessage: V1.Connect, host: Host, speaks?: Record<number, Speak>) {
  const { port1, port2 } = new MessageChannel();
  const got = new Promise<unknown>((resolve) => (port1.onmessage = (e) => resolve(e.data)));
  const result = answer(connectMessage, port2, host, speaks);
  const message = await got;
  port1.close();
  return { result, message };
}

describe("the handshake", () => {
  it("welcomes the published client in protocol 1, with where it is and what's ws's", async () => {
    const { client, session } = await connected();
    expect(client.version).toBe(1);
    expect(session.version).toBe(1);
    expect(client.name).toBe("jira");
    expect(client.path).toBe("/things/2?tab=links");
    expect(client.base).toBe("/plugin-frame/jira");
    expect(client.api("/issues")).toBe("/plugins/jira/issues");
    expect(client.theme).toEqual({ scheme: "dark", vars: vars("dark") });
    client.close();
    session.close();
  });

  it("sends a welcome the protocol's own guard takes, with every token and ws's keys", async () => {
    const { message } = await answered({ type: "ws:connect", versions: [1] }, fakeWs().host);
    expect(V1.isWelcome(message)).toBe(true);
    const welcome = message as V1.Welcome;
    expect(welcome.ws).toBe("1.2.3");
    expect(Object.keys(welcome.theme.vars).sort()).toEqual(TOKENS.map(tokenVar).sort());
    expect(welcome.keys).toEqual(frameKeys);
    expect(welcome.keys.chords).toContain("k");
  });

  it("refuses a page that speaks only a later version, saying ws is behind", async () => {
    const { result, message } = await answered({ type: "ws:connect", versions: [2] }, fakeWs().host);
    expect(isRefuse(message)).toBe(true);
    expect(message).toEqual({ type: "ws:refuse", versions: [1], message: "jira speaks frame protocol 2, and this ws speaks 1. Update ws." });
    expect(result).toEqual({ refused: "jira speaks frame protocol 2, and this ws speaks 1. Update ws." });
  });

  it("refuses a page that speaks only versions ws has dropped, saying the plugin is behind", () => {
    expect(refusal("jira", [1], [2, 3])).toBe("jira speaks frame protocol 1, and this ws speaks 2, 3. Update the plugin.");
    expect(refusal("jira", [], [1])).toBe("jira lists no frame protocol version, and this ws speaks 1. Update the plugin.");
  });

  it("gives the published client a ProtocolError when they have no version in common", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    const ws = fakeWs();
    // A ws that has gone on to 2 and dropped 1.
    const later: Record<number, Speak> = { 2: () => ({ version: 2, navigate() {}, theme() {}, changed() {}, close() {} }) };
    const failed = connect({ host: { postMessage: (m, _o, [port]) => void answer(m as V1.Connect, port as MessagePort, ws.host, later) } });
    await expect(failed).rejects.toBeInstanceOf(ProtocolError);
    await expect(failed).rejects.toMatchObject({ page: [1], host: [2], message: "jira speaks frame protocol 1, and this ws speaks 2. Update the plugin." });
    error.mockRestore();
  });

  it("keeps answering an old version once it speaks a newer one", async () => {
    const ws = fakeWs();
    const spoke: number[] = [];
    const two: Speak = (port) => {
      spoke.push(2);
      port.postMessage({ type: "ws:welcome", version: 2 });
      return { version: 2, navigate() {}, theme() {}, changed() {}, close: () => port.close() };
    };
    const both: Record<number, Speak> = { 1: v1, 2: two };

    // A page built for 1 is welcomed in 1, as it always was.
    const old = await answered({ type: "ws:connect", versions: [1] }, ws.host, both);
    expect(V1.isWelcome(old.message)).toBe(true);
    expect("session" in old.result && old.result.session.version).toBe(1);
    // One that speaks both gets the higher.
    const newer = await answered({ type: "ws:connect", versions: [1, 2] }, ws.host, both);
    expect(newer.message).toEqual({ type: "ws:welcome", version: 2 });
    expect(spoke).toEqual([2]);
  });

  it("speaks protocol 1", () => {
    expect(versions[1]).toBe(v1);
  });
});

describe("protocol 1's host", () => {
  it("hears where the app went, what it's called, what it opens and the keys it passes on", async () => {
    const { client, session, said } = await connected();
    client.setPath("/things/3");
    client.setTitle("Thing 3");
    client.setTitle("");
    client.open("/missions/token-cache");
    await tick();
    expect(said.moved).toEqual(["/things/3"]);
    expect(said.titled).toEqual(["Thing 3", ""]);
    expect(said.opened).toEqual(["/missions/token-cache"]);
    client.close();
    session.close();
  });

  it("tells the app when ws's address moved, the theme changed, and the plugin said it's stale", async () => {
    const { client, session, setTheme } = await connected();
    const got = { navigated: [] as string[], themes: [] as V1.Theme[], changed: 0 };
    client.onNavigate((p) => got.navigated.push(p));
    client.onTheme((t) => got.themes.push(t));
    client.onChanged(() => got.changed++);

    session.navigate("/board");
    setTheme({ scheme: "light", vars: vars("light") });
    session.theme();
    session.changed();
    await tick();

    expect(got.navigated).toEqual(["/board"]);
    expect(client.path).toBe("/board");
    // The theme sent is ws's as it is when it changed, every token of it.
    expect(got.themes).toEqual([{ scheme: "light", vars: vars("light") }]);
    expect(client.theme.scheme).toBe("light");
    expect(got.changed).toBe(1);
    client.close();
    session.close();
  });

  it("passes on a key the page says was ws's, and drops what it doesn't know", async () => {
    const ws = fakeWs();
    const { port1, port2 } = new MessageChannel();
    const session = v1(port2, ws.host);
    port1.postMessage({ type: "ws:key", key: "k", meta: true, ctrl: false, shift: false, alt: false });
    port1.postMessage({ type: "ws:room", width: 1200 }); // a message from a later release of the package
    port1.postMessage({ type: "ws:path" }); // not as the protocol has it
    port1.postMessage("ws:title");
    await tick();
    expect(ws.said.keys).toEqual([{ key: "k", meta: true, ctrl: false, shift: false, alt: false }]);
    expect(ws.said.moved).toEqual([]);
    expect(ws.said.titled).toEqual([]);
    session.close();
    port1.close();
  });
});

// listen is the window's side: which messages are a plugin's page
// connecting, and which are somebody else's.
describe("listening for the frame", () => {
  afterEach(() => vi.unstubAllGlobals());

  function setup() {
    const win = Object.assign(new EventTarget(), { location: { origin: "http://127.0.0.1:4000" } });
    vi.stubGlobal("window", win);
    const frameWindow = {} as Window;
    const ws = fakeWs();
    const refusals: (string | undefined)[] = [];
    const frame = listen(
      () => frameWindow,
      ws.host,
      (m) => refusals.push(m),
    );
    // A message as the window gets it, with the page's end of a channel.
    const post = (data: unknown, from: { source?: unknown; origin?: string; port?: boolean } = {}) => {
      const { port1, port2 } = new MessageChannel();
      const got: unknown[] = [];
      port1.onmessage = (e) => got.push(e.data);
      const ports = from.port === false ? [] : [port2];
      win.dispatchEvent(Object.assign(new Event("message"), { data, source: from.source ?? frameWindow, origin: from.origin ?? "http://127.0.0.1:4000", ports }));
      return { got, close: () => port1.close(), port: port1 };
    };
    return { frame, ws, refusals, post };
  }
  const hello = { type: "ws:connect", versions: [1], client: "@withworkspace/ui-lib@0.1.0" };

  it("answers the frame's own window, from this origin, with a port, and nobody else", async () => {
    const { frame, post } = setup();
    const others = [
      post(hello, { source: {} }), // another window
      post(hello, { origin: "http://elsewhere.example" }),
      post(hello, { port: false }),
      post({ type: "ws:ready" }), // a page from before the frame protocol
      post({ type: "ws:connect" }), // no versions
    ];
    const page = post(hello);
    await tick();
    for (const other of others) expect(other.got).toEqual([]);
    expect(page.got).toHaveLength(1);
    expect(V1.isWelcome(page.got[0])).toBe(true);
    frame.stop();
    [...others, page].forEach((p) => p.close());
  });

  it("speaks to the page that connected last, and hangs up on the one before: each load of the frame connects again", async () => {
    const { frame, ws, post } = setup();
    const first = post(hello);
    await tick();
    const second = post(hello);
    await tick();
    frame.navigate("/board");
    frame.changed();
    // The page before is gone: what's still to come from it isn't heard.
    first.port.postMessage({ type: "ws:path", path: "/from-the-page-before" });
    second.port.postMessage({ type: "ws:path", path: "/things/3" });
    await tick();
    expect(first.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome"]);
    expect(second.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome", "ws:navigate", "ws:changed"]);
    expect(ws.said.moved).toEqual(["/things/3"]);
    frame.stop();
    first.close();
    second.close();
  });

  it("says what it refused, and takes it back when a page connects", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { frame, refusals, post } = setup();
    const ahead = post({ ...hello, versions: [2] });
    await tick();
    expect(refusals).toEqual(["jira speaks frame protocol 2, and this ws speaks 1. Update ws."]);
    expect(ahead.got).toEqual([{ type: "ws:refuse", versions: [1], message: refusals[0] }]);
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("@withworkspace/ui-lib@0.1.0"));
    // With nobody connected, there's nobody to tell: nothing's thrown.
    frame.navigate("/board");
    const page = post(hello);
    await tick();
    expect(refusals).toEqual([refusals[0], undefined]);
    frame.stop();
    ahead.close();
    page.close();
    warn.mockRestore();
  });

  it("stops: a page connecting afterwards isn't answered, and the one connected is hung up on", async () => {
    const { frame, post } = setup();
    const page = post(hello);
    await tick();
    frame.stop();
    frame.changed();
    const late = post(hello);
    await tick();
    expect(page.got).toHaveLength(1);
    expect(late.got).toEqual([]);
    page.close();
    late.close();
  });
});
