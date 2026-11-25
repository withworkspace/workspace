import { describe, expect, it } from "vitest";
import type { TreeFile } from "../../api";
import { leavesOf } from "../../ui/treeNodes";
import { ancestors, changedIn, filesTree } from "./tree";

const files: TreeFile[] = [
  { path: "internal/web/api.go", status: "modified" },
  { path: "internal/web/files.go" },
  { path: "internal/nav/nav.go", status: "added", uncommitted: true },
  { path: "cmd/demo/main.go" },
  { path: "old.go", status: "deleted" },
  { path: "README.md" },
];

describe("the explorer's tree", () => {
  const nodes = filesTree(files);

  it("has a folder that holds only a folder as one row, folders first", () => {
    expect(nodes.map((n) => n.id)).toEqual(["cmd/demo", "internal", "old.go", "README.md"]);
    expect(nodes[0].name).toBe("cmd/demo");
  });

  it("keeps each file on its row, in the order the tree shows them", () => {
    expect(leavesOf(nodes).map((n) => n.value?.path)).toEqual(["cmd/demo/main.go", "internal/nav/nav.go", "internal/web/api.go", "internal/web/files.go", "old.go", "README.md"]);
  });

  it("counts the changed files in each folder", () => {
    const counts = changedIn(nodes);
    expect(counts.get("internal")).toBe(2);
    expect(counts.get("internal/web")).toBe(1);
    expect(counts.get("cmd/demo")).toBe(0);
  });

  it("names the folders a path is in", () => {
    expect(ancestors("a/b/c.go")).toEqual(["a", "a/b"]);
    expect(ancestors("c.go")).toEqual([]);
  });
});
