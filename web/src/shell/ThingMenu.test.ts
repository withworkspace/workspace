import { describe, expect, it } from "vitest";
import type { Workspace } from "../api";
import { actsAt } from "./ThingMenu";

const headline = { group: "working", kind: "working", label: "", text: "", action: {} };
const workspace = {
  name: "test",
  root: "/work/ws",
  org: "acme",
  missions: [{ slug: "ready", title: "Ready", state: "review", created: "", updated: "", headline, capsules: [], crew: [] }],
  conversations: [{ id: "w1", title: "Open one", opening: "", status: "open", at: "", updated: "" }],
  pages: [{ id: "p1", title: "Notes" }],
  artifacts: [{ id: "a1", title: "Many", status: "live", versions: 3 }],
} as unknown as Workspace;

const titles = (to: string) => actsAt(to, workspace).map((it) => it.title);

describe("what a row's menu can do", () => {
  it("is what Find can do to the thing the row goes to", () => {
    expect(titles("/missions/ready")).toEqual(["Park", "Archive", "Open brief"]);
    expect(titles("/conversations/w1")).toEqual(["Resolve"]);
    expect(titles("/artifacts/a1")).toEqual(["Compare versions"]);
    expect(titles("/reviews/demo/12")[0]).toBe("Dock here");
  });

  it("is nothing for a row that goes to something with nothing to do to it", () => {
    expect(titles("/pages/p1")).toEqual([]);
    expect(titles("/missions/gone")).toEqual([]);
    expect(titles("/documents/records/docs/plan.md")).toEqual([]);
    expect(titles("https://github.com/acme/demo/pull/12")).toEqual([]);
    expect(actsAt("/missions/ready", undefined)).toEqual([]);
  });
});
