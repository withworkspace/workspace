import { describe, expect, it } from "vitest";
import { askAt, newTalkAt, paneAt, peekAt, talkAt } from "./panes";

describe("what has a pane", () => {
  it("is a conversation, the workspace's or about a mission", () => {
    expect(paneAt("/conversations/w1")).toEqual({ kind: "conversation", slug: "", id: "w1" });
    expect(paneAt("/missions/dark-mode/conversations/t%201")).toEqual({ kind: "conversation", slug: "dark-mode", id: "t 1" });
    expect(paneAt("/conversations/w1?turn=3#reply")).toEqual({ kind: "conversation", slug: "", id: "w1" });
  });

  it("is a conversation about a thing that hasn't begun", () => {
    expect(paneAt("/ask/THING-2")).toEqual({ kind: "ask", thing: "THING-2" });
  });

  it("is a conversation about nothing that hasn't begun, which no conversation's own address is", () => {
    expect(paneAt(newTalkAt)).toEqual({ kind: "new" });
    expect(paneAt(talkAt("w1"))).toEqual({ kind: "conversation", slug: "", id: "w1" });
  });

  it("is a mission, whichever of its tabs the address names", () => {
    expect(paneAt("/missions/dark-mode")).toEqual({ kind: "mission", slug: "dark-mode" });
    expect(paneAt("/missions/dark-mode?tab=work")).toEqual({ kind: "mission", slug: "dark-mode" });
  });

  it("is a document, on its repo's default branch or in a capsule", () => {
    expect(paneAt("/documents/records/adr/3.md")).toEqual({ kind: "document", ref: "records:adr/3.md" });
    expect(paneAt("/documents/records/adr/a%20b.md?in=tidy#why")).toEqual({ kind: "document", ref: "records/tidy:adr/a b.md" });
  });

  it("is a pull request", () => {
    expect(paneAt("/reviews/api/42")).toEqual({ kind: "pr", repo: "api", number: 42 });
  });

  it("isn't a page with no narrow form yet", () => {
    expect(paneAt("/conversations")).toBeUndefined();
    expect(paneAt("/missions/dark-mode/review/api/tidy")).toBeUndefined();
    expect(paneAt("/documents/records")).toBeUndefined();
    expect(paneAt("/documents/local/p8")).toBeUndefined();
    expect(paneAt("/reviews")).toBeUndefined();
    expect(paneAt("/reviews/api/stack")).toBeUndefined();
  });
});

describe("what a link opens when it's peeked at", () => {
  const origin = "http://ws.localhost:4000";

  it("is the thing it goes to, without where in it", () => {
    expect(peekAt(`${origin}/missions/dark-mode?tab=verify#b2`, origin)).toBe("/missions/dark-mode");
    expect(peekAt("/reviews/api/42?view=changes", origin)).toBe("/reviews/api/42");
    expect(peekAt("/conversations/w1?turn=3", origin)).toBe("/conversations/w1");
  });

  it("keeps the capsule a document is in, and not that it's being edited", () => {
    expect(peekAt("/documents/records/adr/3.md?edit=p8", origin)).toBe("/documents/records/adr/3.md");
    expect(peekAt("/documents/records/adr/3.md?in=tidy&edit#why", origin)).toBe("/documents/records/adr/3.md?in=tidy");
    expect(paneAt(peekAt("/documents/records/adr/a%20b.md?in=tidy", origin)!)).toEqual({ kind: "document", ref: "records/tidy:adr/a b.md" });
  });

  it("is nothing for a link out of ws, or to what has no pane", () => {
    expect(peekAt("https://github.com/withworkspace/workspace/pull/42", origin)).toBeUndefined();
    expect(peekAt("/settings/about", origin)).toBeUndefined();
    expect(peekAt("/ask/THING-2", origin)).toBeUndefined();
    expect(peekAt(newTalkAt, origin)).toBeUndefined();
    expect(peekAt("/files/api", origin)).toBeUndefined();
  });
});

describe("a tab's address", () => {
  it("is the conversation's own page's, and comes back as it went", () => {
    expect(talkAt("w1")).toBe("/conversations/w1");
    expect(paneAt(talkAt("t 1", "dark-mode"))).toEqual({ kind: "conversation", slug: "dark-mode", id: "t 1" });
    expect(paneAt(askAt("THING-2"))).toEqual({ kind: "ask", thing: "THING-2" });
  });
});
