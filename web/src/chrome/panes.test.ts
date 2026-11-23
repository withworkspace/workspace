import { describe, expect, it } from "vitest";
import { askAt, paneAt, talkAt } from "./panes";

describe("what has a pane", () => {
  it("is a conversation, the workspace's or about a mission", () => {
    expect(paneAt("/conversations/w1")).toEqual({ kind: "conversation", slug: "", id: "w1" });
    expect(paneAt("/missions/dark-mode/conversations/t%201")).toEqual({ kind: "conversation", slug: "dark-mode", id: "t 1" });
    expect(paneAt("/conversations/w1?turn=3#reply")).toEqual({ kind: "conversation", slug: "", id: "w1" });
  });

  it("is a conversation about a thing that hasn't begun", () => {
    expect(paneAt("/ask/THING-2")).toEqual({ kind: "ask", thing: "THING-2" });
  });

  it("isn't a page with no narrow form yet", () => {
    expect(paneAt("/missions/dark-mode")).toBeUndefined();
    expect(paneAt("/conversations")).toBeUndefined();
    expect(paneAt("/documents/records/adr/3.md")).toBeUndefined();
  });
});

describe("a tab's address", () => {
  it("is the conversation's own page's, and comes back as it went", () => {
    expect(talkAt("w1")).toBe("/conversations/w1");
    expect(paneAt(talkAt("t 1", "dark-mode"))).toEqual({ kind: "conversation", slug: "dark-mode", id: "t 1" });
    expect(paneAt(askAt("THING-2"))).toEqual({ kind: "ask", thing: "THING-2" });
  });
});
