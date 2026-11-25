import { describe, expect, it } from "vitest";
import { compact, filterTree, foldersOf, leavesOf, pathTo, shownIds, type TreeNode, treeOf } from "./treeNodes";

const paths = ["web/src/ui/file10.ts", "web/src/ui/file2.ts", "web/package.json", "README.md", "cmd/workspace/main.go", "internal/nav/nav.go", "internal/web/api.go"];
const tree = () => treeOf(paths, (p) => p);

// names is the tree as words: a folder with what's in it in brackets.
const names = (nodes: TreeNode[]): string => nodes.map((n) => (n.children ? `${n.name}[${names(n.children)}]` : n.name)).join(" ");

describe("treeOf", () => {
  it("puts each path in its folders, folders first, by name with numbers counted", () => {
    expect(names(tree())).toBe("cmd[workspace[main.go]] internal[nav[nav.go] web[api.go]] web[src[ui[file2.ts file10.ts]] package.json] README.md");
  });

  it("gives a folder its path as its id, and a file what it was made from", () => {
    const web = tree().find((n) => n.name === "web")!;
    expect(web.id).toBe("web");
    expect(web.children![0]!.id).toBe("web/src");
    expect(tree().find((n) => n.id === "README.md")!.value).toBe("README.md");
  });
});

describe("compact", () => {
  it("shows a folder that holds only a folder as one row, by the inner one's id", () => {
    const c = compact(tree());
    expect(names(c)).toBe("cmd/workspace[main.go] internal[nav[nav.go] web[api.go]] web[src/ui[file2.ts file10.ts] package.json] README.md");
    expect(c[0]!.id).toBe("cmd/workspace");
  });

  it("leaves the tree it was given as it was", () => {
    const t = tree();
    compact(t);
    expect(t[0]!.name).toBe("cmd");
  });
});

describe("filterTree", () => {
  it("keeps the rows asked for and the folders they're in", () => {
    expect(names(filterTree(tree(), (n) => n.id.endsWith(".go") && n.id.includes("nav")))).toBe("internal[nav[nav.go]]");
  });

  it("keeps all of a folder that's kept itself", () => {
    expect(names(filterTree(tree(), (n) => n.id === "web/src"))).toBe("web[src[ui[file2.ts file10.ts]]]");
  });

  it("is empty when nothing's kept", () => {
    expect(filterTree(tree(), () => false)).toEqual([]);
  });
});

describe("asking about a tree", () => {
  it("names every folder, and every row that isn't one", () => {
    expect(foldersOf(tree())).toEqual(["cmd", "cmd/workspace", "internal", "internal/nav", "internal/web", "web", "web/src", "web/src/ui"]);
    expect(leavesOf(tree()).map((n) => n.id)).toHaveLength(paths.length);
  });

  it("says which folders a row is in, outermost first", () => {
    expect(pathTo(tree(), "web/src/ui/file2.ts")).toEqual(["web", "web/src", "web/src/ui"]);
    expect(pathTo(compact(tree()), "web/src/ui/file2.ts")).toEqual(["web", "web/src/ui"]);
    expect(pathTo(tree(), "README.md")).toEqual([]);
    expect(pathTo(tree(), "nope")).toBeUndefined();
  });

  it("lists the rows that show with some folders open", () => {
    expect(shownIds(tree(), new Set())).toEqual(["cmd", "internal", "web", "README.md"]);
    expect(shownIds(tree(), new Set(["internal", "internal/nav"]))).toEqual(["cmd", "internal", "internal/nav", "internal/nav/nav.go", "internal/web", "web", "README.md"]);
  });
});
