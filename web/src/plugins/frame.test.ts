import { connect, ProtocolError } from "@withworkspace/ui-lib";
import { isRefuse, TOKENS, V1 } from "@withworkspace/ui-lib/protocol";
import { afterEach, describe, expect, it, vi } from "vitest";
import { frameKeys } from "../keys";
import type { FoundRef } from "../refs/found";
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
  const said = { went: [] as string[], titled: [] as string[], keys: [] as KeyPress[], asked: [] as string[][], dims: [] as boolean[], heres: [] as (string | undefined)[], layouts: [] as string[] };
  let theme: V1.Theme = { scheme: "dark", vars: vars("dark") };
  // What ws finds: THING-<n>, wherever it's written, until a test says
  // otherwise.
  let find = (text: string): FoundRef[] =>
    [...text.matchAll(/THING-\d+/g)].map((m) => ({ from: m.index, to: m.index + m[0].length, ref: JSON.stringify({ kind: "thing", id: m[0] }), kind: "thing", href: `/x/jira/things/${m[0]}` }));
  const host: Host = {
    plugin: "jira",
    ws: "1.2.3",
    path: () => path,
    theme: () => theme,
    titled: (t) => void said.titled.push(t),
    go: (p, replace) => void said.went.push(replace ? `${p} (replace)` : p),
    key: (k) => void said.keys.push(k),
    refs: (texts) => {
      said.asked.push(texts);
      return texts.map(find);
    },
    dim: (on) => void said.dims.push(on),
    here: (name) => void said.heres.push(name),
    chromeLayout: (mode) => void said.layouts.push(mode),
  };
  return { host, said, setTheme: (t: V1.Theme) => void (theme = t), setFind: (f: typeof find) => void (find = f) };
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
    expect(client.href("/things/3")).toBe("/x/jira/things/3");
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
    const later: Record<number, Speak> = { 2: () => ({ version: 2, navigate() {}, theme() {}, changed() {}, refsChanged() {}, outside() {}, close() {} }) };
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
      return { version: 2, navigate() {}, theme() {}, changed() {}, refsChanged() {}, outside() {}, close: () => port.close() };
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
  it("hears where the app asks to go, as ws's address, and what it's called", async () => {
    const { client, session, said } = await connected();
    client.go("/things/3");
    client.go("/board?q=cache", { replace: true });
    client.setTitle("Thing 3");
    client.setTitle("");
    client.open("/missions/token-cache");
    await tick();
    expect(said.went).toEqual(["/x/jira/things/3", "/x/jira/board?q=cache (replace)", "/missions/token-cache"]);
    expect(said.titled).toEqual(["Thing 3", ""]);
    // The app hasn't moved: it moves when ws says so.
    expect(client.path).toBe("/things/2?tab=links");
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

  it("says it finds names, and finds the ones in the words the app sends, each text's in its turn", async () => {
    const { client, session, said } = await connected();
    const [first, second] = await Promise.all([client.findRefs(["THING-2 goes beside THING-1.", "Nothing here"]), client.findRefs(["About THING-3"])]);
    expect(first).toEqual([
      [
        { from: 0, to: 7, ref: '{"kind":"thing","id":"THING-2"}', kind: "thing", href: "/x/jira/things/THING-2" },
        { from: 20, to: 27, ref: '{"kind":"thing","id":"THING-1"}', kind: "thing", href: "/x/jira/things/THING-1" },
      ],
      [],
    ]);
    expect(second).toEqual([[{ from: 6, to: 13, ref: '{"kind":"thing","id":"THING-3"}', kind: "thing", href: "/x/jira/things/THING-3" }]]);
    // The client's askings of one turn came as one.
    expect(said.asked).toEqual([["THING-2 goes beside THING-1.", "Nothing here", "About THING-3"]]);
    client.close();
    session.close();
  });

  it("sends what it found as the protocol has a ref, whatever else ws knows of it", async () => {
    const ws = fakeWs();
    ws.setFind(() => [{ from: 0, to: 4, ref: '{"kind":"repo","repo":"demo"}', kind: "repo", href: "/files/demo", shape: "pill", accent: "#2f7d5b" }]);
    const { port1, port2 } = new MessageChannel();
    const session = v1(port2, ws.host);
    const got: unknown[] = [];
    port1.onmessage = (e) => got.push(e.data);
    port1.postMessage({ type: "ws:refs", id: 4, texts: ["demo"] } satisfies V1.Refs);
    port1.postMessage({ type: "ws:refs", texts: ["demo"] }); // not as the protocol has it: no id to answer by
    await tick();
    expect(V1.isWelcome(got[0]) && got[0].refs).toBe(true);
    expect(got.slice(1)).toEqual([{ type: "ws:refs-found", id: 4, found: [[{ from: 0, to: 4, ref: '{"kind":"repo","repo":"demo"}', kind: "repo", href: "/files/demo", shape: "pill", accent: "#2f7d5b" }]] }]);
    expect(got.slice(1).every(V1.isHostMessage)).toBe(true);
    session.close();
    port1.close();
  });

  it("tells an app that has asked when what there is to name changed, and an app that hasn't nothing", async () => {
    const { client, session, setFind } = await connected();
    let changed = 0;
    client.onRefsChanged(() => changed++);

    // It hasn't asked: it drew no names, and has none to draw again.
    session.refsChanged();
    await tick();
    expect(changed).toBe(0);

    expect(await client.findRefs(["mission-one is under way"])).toEqual([[]]);
    setFind((text) => (text.startsWith("mission-one") ? [{ from: 0, to: 11, ref: '{"kind":"mission","slug":"mission-one"}', kind: "mission", href: "/missions/mission-one" }] : []));
    session.refsChanged();
    await tick();
    expect(changed).toBe(1);
    expect((await client.findRefs(["mission-one is under way"]))[0]).toEqual([{ from: 0, to: 11, ref: '{"kind":"mission","slug":"mission-one"}', kind: "mission", href: "/missions/mission-one" }]);
    client.close();
    session.close();
  });

  it("tells the app of a press in ws, outside the frame, whether it asked or not", async () => {
    const { client, session } = await connected();
    let presses = 0;
    const twice = new Promise<void>((resolve) => client.onOutside(() => ++presses === 2 && resolve()));
    session.outside();
    session.outside();
    await twice;
    expect(presses).toBe(2);
    client.close();
    session.close();
  });

  it("hears what the app says it shows beyond its address, and none once it's taken off", async () => {
    const { client, session, said } = await connected();
    const panel = client.pushHere("THING-2");
    await tick();
    panel();
    await tick();
    expect(said.heres).toEqual(["THING-2", undefined]);
    client.close();
    session.close();
  });

  it("hears how the app wants the chrome laid out", async () => {
    const { client, session, said } = await connected();
    client.setChromeLayout("expanded");
    await tick();
    client.setChromeLayout("standard");
    await tick();
    expect(said.layouts).toEqual(["expanded", "standard"]);
    client.close();
    session.close();
  });

  it("passes on a key the page says was ws's, and drops what it doesn't know", async () => {
    const ws = fakeWs();
    const { port1, port2 } = new MessageChannel();
    const session = v1(port2, ws.host);
    port1.postMessage({ type: "ws:key", key: "k", meta: true, ctrl: false, shift: false, alt: false });
    port1.postMessage({ type: "ws:sidebar", open: false }); // a message from a later release of the package
    port1.postMessage({ type: "ws:go" }); // not as the protocol has it
    port1.postMessage({ type: "ws:path", path: "/things/3" }); // from before ws kept the history
    port1.postMessage("ws:title");
    await tick();
    expect(ws.said.keys).toEqual([{ key: "k", meta: true, ctrl: false, shift: false, alt: false }]);
    expect(ws.said.went).toEqual([]);
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
    // The frame's window is the same one from load to load; its document
    // is each page's own.
    const frameWindow = { document: {} } as Window;
    const load = () => void Object.assign(frameWindow, { document: {} });
    const ws = fakeWs();
    const refusals: (string | undefined)[] = [];
    // The watch on the connected page for a backdrop, fired by hand: what
    // each page was watched with, and whether its watch was stopped.
    const watched: { page: Document; backdrop: (on: boolean) => void; stopped: boolean }[] = [];
    const frame = listen(
      () => frameWindow,
      ws.host,
      (m) => refusals.push(m),
      (page, said) => {
        const w = { page, backdrop: said, stopped: false };
        watched.push(w);
        return () => void (w.stopped = true);
      },
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
    return { frame, ws, refusals, post, load, watched };
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
    first.port.postMessage({ type: "ws:go", path: "/x/jira/from-the-page-before" });
    second.port.postMessage({ type: "ws:go", path: "/x/jira/things/3" });
    await tick();
    expect(first.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome"]);
    expect(second.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome", "ws:navigate", "ws:changed"]);
    expect(ws.said.went).toEqual(["/x/jira/things/3"]);
    frame.stop();
    first.close();
    second.close();
  });

  it("says a press outside to the page that's connected, and to nobody while none is", async () => {
    const { frame, post } = setup();
    frame.outside();
    const page = post(hello);
    await tick();
    frame.outside();
    await tick();
    expect(page.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome", "ws:outside"]);
    frame.stop();
    page.close();
  });

  it("watches the page that's connected for a backdrop, and dims while it has one", async () => {
    const { frame, ws, post, watched } = setup();
    expect(watched).toEqual([]);
    const page = post(hello);
    await tick();
    expect(watched).toHaveLength(1);
    watched[0].backdrop(true);
    watched[0].backdrop(false);
    expect(ws.said.dims).toEqual([true, false]);
    frame.stop();
    page.close();
  });

  it("ends the dim when the page with the backdrop is hung up on: another connects, or ws stops listening", async () => {
    const { frame, ws, post, watched } = setup();
    const first = post(hello);
    await tick();
    watched[0].backdrop(true);
    expect(ws.said.dims).toEqual([true]);

    // The frame loaded again and its page connected: the one before went
    // with its backdrop up, and isn't watched any more.
    const second = post(hello);
    await tick();
    expect(ws.said.dims).toEqual([true, false]);
    expect(watched.map((w) => w.stopped)).toEqual([true, false]);

    watched[1].backdrop(true);
    frame.stop();
    expect(ws.said.dims).toEqual([true, false, true, false]);
    // What either page said it showed is forgotten with it, as the dim is,
    // and its layout is standard again.
    expect(ws.said.heres).toEqual([undefined, undefined]);
    expect(ws.said.layouts).toEqual(["standard", "standard"]);
    expect(watched.map((w) => w.stopped)).toEqual([true, true]);
    first.close();
    second.close();
  });

  it("hangs up on a page the frame has loaded past, and leaves the one it's showing", async () => {
    const { frame, ws, post, load, watched } = setup();
    const page = post(hello);
    await tick();
    watched[0].backdrop(true);

    // The frame's load, for the page that's connected: it's left.
    frame.loaded();
    frame.outside();
    await tick();
    expect(ws.said.dims).toEqual([true]);
    expect(page.got.map((m) => (m as { type: string }).type)).toEqual(["ws:welcome", "ws:outside"]);

    // The frame loaded another page, which didn't connect (its bundle
    // failed, say): the page before is gone, and so is its dim.
    load();
    frame.loaded();
    frame.outside();
    await tick();
    expect(ws.said.dims).toEqual([true, false]);
    expect(watched[0].stopped).toBe(true);
    expect(page.got).toHaveLength(2);
    frame.stop();
    page.close();
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
