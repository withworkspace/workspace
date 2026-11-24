// The page's socket: one per tab, over which it follows every topic it shows
// and is told when one has changed (internal/web/socket.go). A browser
// allows a few connections to a server, shared by every tab; a stream held
// for each topic used them up, and a socket is one whatever a page follows.

import type { SocketAsk, SocketSaid } from "./wire.gen";

// Follower is what a part of a page does as it follows a topic: fetch what
// it shows once it's followed (so nothing that changes after is missed),
// fetch again on each change, and know when it's been lost (closed: refused,
// or the socket couldn't open at all, as when it's signed out).
export interface Follower {
  following?: () => void;
  // changed is told what the server said: for a topic whose followers hold
  // what it shows, how it changed (said.conversation), when it can say.
  changed: (said: SocketSaid) => void;
  lost?: (closed: boolean) => void;
}

interface Followed {
  app: string;
  followers: Set<Follower>;
  // following is whether the server has said it's followed since the
  // socket last opened: a follower that comes after is told at once.
  following: boolean;
}

// The socket is closed this long after nothing's followed (the page's
// hidden, say), so a quick return doesn't open it again, and a hidden tab
// doesn't hold the daemon up.
const idleClose = 5_000;
// Opening again after it's lost waits this long, doubling each time up to
// retryMost.
const retryFirst = 500;
const retryMost = 10_000;

export class PageSocket {
  private ws?: WebSocket;
  private open = false;
  private retry = retryFirst;
  private reopen?: ReturnType<typeof setTimeout>;
  private idle?: ReturnType<typeof setTimeout>;
  private topics = new Map<string, Followed>();

  // follow follows topic for f, until what it returns is called. app is the
  // plugin whose app the page shows, for the plugins topic: following again
  // with another follows it anew.
  follow(topic: string, f: Follower, app = ""): () => void {
    clearTimeout(this.idle);
    let t = this.topics.get(topic);
    if (t && t.app !== app) {
      t.app = app;
      t.following = false;
      this.send({ op: "leave", topic });
      this.send({ op: "follow", topic, app });
    }
    if (!t) {
      t = { app, followers: new Set(), following: false };
      this.topics.set(topic, t);
      this.send({ op: "follow", topic, app });
    } else if (t.following) {
      queueMicrotask(() => f.following?.());
    }
    t.followers.add(f);
    this.connect();
    return () => {
      const now = this.topics.get(topic);
      if (!now) return;
      now.followers.delete(f);
      if (now.followers.size > 0) return;
      this.topics.delete(topic);
      this.send({ op: "leave", topic });
      if (this.topics.size === 0) {
        clearTimeout(this.idle);
        this.idle = setTimeout(() => this.close(), idleClose);
      }
    };
  }

  private send(ask: SocketAsk) {
    if (!ask.app) delete ask.app;
    if (this.open) this.ws?.send(JSON.stringify(ask));
  }

  private connect() {
    if (this.ws || this.reopen !== undefined) return;
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${scheme}://${location.host}/api/socket`);
    this.ws = ws;
    let opened = false;
    ws.onopen = () => {
      opened = true;
      this.open = true;
      this.retry = retryFirst;
      for (const [topic, t] of this.topics) {
        t.following = false;
        this.send({ op: "follow", topic, app: t.app });
      }
    };
    ws.onmessage = (e) => {
      let said: SocketSaid;
      try {
        said = JSON.parse(String(e.data));
      } catch {
        return;
      }
      const t = this.topics.get(said.topic);
      if (!t) return;
      switch (said.kind) {
        case "following":
          t.following = true;
          for (const f of [...t.followers]) f.following?.();
          break;
        case "changed":
          for (const f of [...t.followers]) f.changed(said);
          break;
        case "refused":
          for (const f of [...t.followers]) f.lost?.(true);
          break;
      }
    };
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.ws = undefined;
      this.open = false;
      // One that never opened was refused (signed out, say), or there's no
      // daemon: either way its followers find out by reading.
      for (const t of this.topics.values()) {
        t.following = false;
        for (const f of [...t.followers]) f.lost?.(!opened);
      }
      if (this.topics.size === 0) return;
      const wait = this.retry;
      this.retry = Math.min(this.retry * 2, retryMost);
      this.reopen = setTimeout(() => {
        this.reopen = undefined;
        if (this.topics.size > 0) this.connect();
      }, wait);
    };
  }

  private close() {
    if (this.topics.size > 0) return;
    clearTimeout(this.reopen);
    this.reopen = undefined;
    const ws = this.ws;
    this.ws = undefined;
    this.open = false;
    ws?.close();
  }
}

// pageSocket is the tab's one socket.
export const pageSocket = new PageSocket();
