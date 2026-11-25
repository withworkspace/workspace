import { describe, expect, it } from "vitest";
import type { DiffFile } from "../api";
import { groupFiles } from "./files";
import { reviewFilesTree } from "./reviewFilesTree";

const file = (path: string, more: Partial<DiffFile> = {}): DiffFile => ({ path, status: "modified", additions: 1, deletions: 0, hunks: [], ...more });

describe("a review's files as a tree", () => {
  it("has a row for each group, in the order they're read, over its files", () => {
    const files = [file("main.go"), file("internal/render/render.go"), file("internal/render/render_test.go"), file("go.sum", { skim: "lockfile" })];
    const nodes = reviewFilesTree(files, groupFiles(files).groups);
    expect(nodes.map((n) => n.name)).toEqual(["./", "internal/render", "To skim"]);
    expect(nodes[1].children!.map((n) => [n.id, n.name])).toEqual([
      ["1", "render.go"],
      ["2", "render_test.go"],
    ]);
    expect(nodes[2].children!.map((n) => n.id)).toEqual(["3"]);
  });

  it("has only the files when they're one group", () => {
    const files = [file("web/src/a.ts"), file("web/src/b.ts")];
    const nodes = reviewFilesTree(files, groupFiles(files).groups);
    expect(nodes.map((n) => [n.id, n.name, !!n.children])).toEqual([
      ["0", "a.ts", false],
      ["1", "b.ts", false],
    ]);
  });

  it("keeps a row over files that are all to skim", () => {
    const files = [file("go.sum", { skim: "lockfile" })];
    const nodes = reviewFilesTree(files, groupFiles(files).groups);
    expect(nodes.map((n) => n.name)).toEqual(["To skim"]);
  });
});
