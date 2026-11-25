import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Tree } from "./Tree";
import { compact, type TreeNode, treeOf } from "./treeNodes";

const nodes = compact(treeOf(["cmd/workspace/main.go", "internal/nav/nav.go", "internal/web/api.go", "README.md"], (p) => p));

const html = (props: Partial<Parameters<typeof Tree<string>>[0]> = {}, of: TreeNode<string>[] = nodes) => renderToStaticMarkup(h(Tree<string>, { nodes: of, "aria-label": "Files", ...props }));

describe("Tree", () => {
  it("is a tree, named, with the page's class last", () => {
    const out = html({ className: "files" });
    expect(out).toMatch(/role="treegrid"/);
    expect(out).toContain('aria-label="Files"');
    expect(out).toMatch(/class="[^"]*tree[^" ]* files"/);
  });

  it("shows the top rows folded, each folder saying it is", () => {
    const out = html();
    expect(out.match(/role="row"/g)).toHaveLength(3);
    expect(out.match(/aria-expanded="false"/g)).toHaveLength(2);
    expect(out).not.toContain("nav.go");
  });

  it("shows what's in the folders it starts open on, at their level", () => {
    const out = html({ startOpen: ["internal", "internal/nav"] });
    expect(out).toContain("nav.go");
    expect(out).not.toContain("api.go");
    expect(out).toMatch(/aria-level="3"/);
  });

  it("opens what the page says is open, and all of it when asked", () => {
    expect(html({ open: new Set(["cmd/workspace"]) })).toContain("main.go");
    expect(html({ startOpen: "all" }).match(/role="row"/g)).toHaveLength(8);
  });

  it("marks the row that's open in the page, which react-aria's doesn't", () => {
    const out = html({ current: "README.md" });
    expect(out.match(/aria-current="true"/g)).toHaveLength(1);
    expect(html()).not.toContain("aria-current");
  });

  it("draws what the page says of a row: its icon, its mark, its treats, its title", () => {
    const out = html({ look: (n) => (n.children ? {} : { icon: "ICON", mark: "MARK", end: "END", title: `the ${n.name}` }) });
    expect(out).toContain("ICON");
    expect(out).toContain("MARK");
    expect(out).toContain("END");
    expect(out).toContain('title="the README.md"');
  });

  it("says what it's given when it's empty", () => {
    expect(html({ empty: "The capsule is empty." }, [])).toContain("The capsule is empty.");
  });
});
