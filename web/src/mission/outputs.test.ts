import { describe, expect, it } from "vitest";
import { artifactTitle, isArtifact, kindTitle, shortPath } from "./outputs";

describe("artifactTitle", () => {
  it("keeps a label that's a name", () => expect(artifactTitle({ kind: "note", label: "Why rounding moved", ref: "mission:notes/x.md" })).toEqual({ title: "Why rounding moved" }));
  it("splits a review's label", () =>
    expect(artifactTitle({ kind: "review", label: "Review r2 of api/tax-rounding: looks right", ref: "mission:reviews/r2.md" })).toEqual({ title: "Review r2 of api/tax-rounding", desc: "Looks right" }));
  it("names a handoff by who made it", () => expect(artifactTitle({ kind: "handoff", label: "", ref: "mission:handoffs/c1.md", by: "c1" })).toEqual({ title: "c1's handoff", desc: undefined }));
  it("makes a title of a long label's kind and maker", () => {
    const label = "A long sentence that says what the proposal is about, and more than sixty-four letters of it";
    expect(artifactTitle({ kind: "proposal", label, ref: "mission:p.md", by: "c2" })).toEqual({ title: "Proposal by c2", desc: label });
  });
  it("falls back to the file's name", () => expect(artifactTitle({ kind: "doc", label: "", ref: "mission:docs/plan.md" })).toEqual({ title: "plan.md", desc: undefined }));
});

describe("the rest", () => {
  it("names kinds", () => {
    expect(kindTitle("adr")).toBe("Decision records");
    expect(kindTitle("sketch")).toBe("Sketchs");
  });
  it("knows an artifact", () => {
    expect(isArtifact({ kind: "artifact", ref: "a3" })).toBe(true);
    expect(isArtifact({ kind: "artifact", ref: "a0" })).toBe(false);
    expect(isArtifact({ kind: "doc", ref: "a3" })).toBe(false);
  });
  it("shortens a capsule's path", () => expect(shortPath("/Users/x/ws/repos/api/fix")).toBe("repos/api/fix"));
});
