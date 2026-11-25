import { describe, expect, it } from "vitest";
import type { Workspace } from "../../api";
import { fileLinkOf } from "./fileLink";

const workspace = { missions: [{ slug: "greet", capsules: [{ repo: "demo", name: "greet", path: "/w/repos/demo/greet/" }] }] } as unknown as Workspace;

describe("fileLinkOf", () => {
  it("links a file in crew's capsule into its explorer", () => {
    const link = fileLinkOf(workspace, "greet", { capsule: "demo/greet" })!;
    expect(link("/w/repos/demo/greet/greet/greet.go")).toBe("/missions/greet/files/demo/greet?path=greet%2Fgreet.go");
    expect(link("README.md")).toBe("/missions/greet/files/demo/greet?path=README.md");
    expect(link("/elsewhere/x.go")).toBeUndefined();
  });

  it("is none for crew with no capsule of the mission's", () => {
    expect(fileLinkOf(workspace, "greet", { capsule: "demo/other" })).toBeUndefined();
    expect(fileLinkOf(workspace, "greet", undefined)).toBeUndefined();
  });
});
