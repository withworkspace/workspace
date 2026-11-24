import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PageSocket } from "./socket";
import type { SocketAsk, SocketSaid } from "./wire.gen";

// FakeSocket stands in for the browser's WebSocket: what the page sent, and
// a way to say what the server says.
class FakeSocket {
  static made: FakeSocket[] = [];
  sent: SocketAsk[] = [];
  closed = false;
  onopen?: () => void;
  onmessage?: (e: { data: string }) => void;
  onclose?: () => void;
  constructor(readonly url: string) {
    FakeSocket.made.push(this);
  }
  send(data: string) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.closed = true;
    this.onclose?.();
  }
  opens() {
    this.onopen?.();
  }
  says(m: SocketSaid) {
    this.onmessage?.({ data: JSON.stringify(m) });
  }
  drops() {
    this.onclose?.();
  }
}

const last = () => FakeSocket.made[FakeSocket.made.length - 1];
const asked = (s: FakeSocket) => s.sent.map((a) => `${a.op} ${a.topic}${a.app ? ` ${a.app}` : ""}`);

describe("PageSocket", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeSocket.made = [];
    vi.stubGlobal("WebSocket", FakeSocket);
    vi.stubGlobal("location", { protocol: "http:", host: "127.0.0.1:4000" });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("follows every topic over one socket, and tells each follower", () => {
    const p = new PageSocket();
    const ws = { following: vi.fn(), changed: vi.fn() };
    const mission = { following: vi.fn(), changed: vi.fn() };
    p.follow("workspace", ws);
    p.follow("mission:m", mission);
    expect(FakeSocket.made).toHaveLength(1);
    const s = last();
    expect(s.url).toBe("ws://127.0.0.1:4000/api/socket");
    s.opens();
    expect(asked(s)).toEqual(["follow workspace", "follow mission:m"]);
    s.says({ kind: "following", topic: "workspace" });
    s.says({ kind: "changed", topic: "mission:m" });
    expect(ws.following).toHaveBeenCalledTimes(1);
    expect(mission.changed).toHaveBeenCalledTimes(1);
    expect(ws.changed).not.toHaveBeenCalled();
  });

  it("tells a follower that comes later it's followed, and leaves a topic once nobody follows it", async () => {
    const p = new PageSocket();
    const first = { following: vi.fn(), changed: vi.fn() };
    const second = { following: vi.fn(), changed: vi.fn() };
    const leaveFirst = p.follow("workspace", first);
    const s = last();
    s.opens();
    s.says({ kind: "following", topic: "workspace" });
    const leaveSecond = p.follow("workspace", second);
    await Promise.resolve();
    expect(second.following).toHaveBeenCalledTimes(1);
    leaveFirst();
    expect(asked(s)).toEqual(["follow workspace"]);
    leaveSecond();
    expect(asked(s)).toEqual(["follow workspace", "leave workspace"]);
  });

  it("opens again when it's lost, following what it followed, and says it was lost", () => {
    const p = new PageSocket();
    const f = { following: vi.fn(), changed: vi.fn(), lost: vi.fn() };
    p.follow("workspace", f);
    last().opens();
    last().drops();
    expect(f.lost).toHaveBeenCalledWith(false);
    vi.advanceTimersByTime(1000);
    expect(FakeSocket.made).toHaveLength(2);
    last().opens();
    expect(asked(last())).toEqual(["follow workspace"]);
  });

  it("says a socket that never opened was closed: a follower finds out why by reading", () => {
    const p = new PageSocket();
    const f = { changed: vi.fn(), lost: vi.fn() };
    p.follow("workspace", f);
    last().drops();
    expect(f.lost).toHaveBeenCalledWith(true);
  });

  it("follows anew when the page shows another plugin's app", () => {
    const p = new PageSocket();
    p.follow("plugins", { changed: vi.fn() }, "jira");
    const s = last();
    s.opens();
    p.follow("plugins", { changed: vi.fn() }, "linear");
    expect(asked(s)).toEqual(["follow plugins jira", "leave plugins", "follow plugins linear"]);
  });

  it("closes a while after nothing's followed, and not when something is again first", () => {
    const p = new PageSocket();
    const leave = p.follow("workspace", { changed: vi.fn() });
    const s = last();
    s.opens();
    leave();
    vi.advanceTimersByTime(1000);
    p.follow("workspace", { changed: vi.fn() });
    vi.advanceTimersByTime(10_000);
    expect(s.closed).toBe(false);
    expect(FakeSocket.made).toHaveLength(1);
  });

  it("closes once nothing's been followed for a while", () => {
    const p = new PageSocket();
    const leave = p.follow("workspace", { changed: vi.fn() });
    const s = last();
    s.opens();
    leave();
    vi.advanceTimersByTime(5_000);
    expect(s.closed).toBe(true);
    vi.advanceTimersByTime(20_000);
    expect(FakeSocket.made).toHaveLength(1);
  });
});
