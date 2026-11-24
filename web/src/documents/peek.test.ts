import { describe, expect, it } from "vitest";
import type { ArtifactContent } from "../api";
import { docPeekOf, titleOf } from "./peek";

const doc = (over: Partial<ArtifactContent>) => ({ ref: "records:adr/3.md", kind: "doc", state: "landed", added: "", readable: true, text: "", markdown: true, version: "v1", annotatable: false, editable: false, ...over }) as ArtifactContent;

describe("what a document calls itself", () => {
  it("is its first heading, after any front matter", () => {
    expect(titleOf("# Limits\n\nEach key…", "docs/limits.md")).toBe("Limits");
    expect(titleOf("---\nstate: accepted\n---\n\n# Queue the webhooks #\n", "adr/3.md")).toBe("Queue the webhooks");
  });

  it("is its file's name when it doesn't open with one", () => {
    expect(titleOf("Each key has so many requests a minute.\n\n# Later\n", "docs/limits.md")).toBe("limits.md");
  });
});

describe("a document's peek", () => {
  it("says where it is, how long it is, and its text without its title", () => {
    const of = docPeekOf(doc({ text: "# Limits\n\nEach key has a limit.\n", in: { repo: "records", worktree: ".ground", path: "adr/3.md" } }), "records:adr/3.md", (r) => r.toUpperCase());
    expect(of).toEqual({ title: "Limits", repo: "RECORDS", capsule: undefined, path: "adr/3.md", words: 5, text: "\nEach key has a limit.\n", mission: undefined });
  });

  it("says the capsule its text is from, as it was asked for where ws doesn't say", () => {
    expect(docPeekOf(doc({ text: "Words." }), "records/tidy:adr/3.md")).toMatchObject({ repo: "records", capsule: "tidy", path: "adr/3.md", title: "3.md" });
  });

  it("leads to the mission that records it", () => {
    const of = docPeekOf(doc({ mission: { slug: "dark-mode", title: "Dark mode", ref: "a3" } }), "records:adr/3.md");
    expect(of.mission).toEqual({ title: "Dark mode", to: "/missions/dark-mode/artifact?ref=a3" });
  });
});
