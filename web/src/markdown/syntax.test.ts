import type { Nodes } from "mdast";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { describe, expect, it } from "vitest";
import { resolveLink, slugger } from "./links";
import { syntax } from "./syntax";

const parser = unified().use(remarkParse).use(syntax);

// kinds is every kind of node in a document's tree, in order.
function kinds(source: string): string[] {
  const out: string[] = [];
  const visit = (n: Nodes) => {
    out.push(n.type);
    if ("children" in n) n.children.forEach(visit);
  };
  visit(parser.parse(source));
  return out;
}

const maths = (source: string) => {
  const out: string[] = [];
  const visit = (n: Nodes) => {
    if (n.type === "inlineMath" || n.type === "math") out.push(n.value);
    if ("children" in n) n.children.forEach(visit);
  };
  visit(parser.parse(source));
  return out;
};

describe("maths", () => {
  it("reads single dollars as maths", () => {
    expect(maths("An exposure score $E_{gh}$ and a weight $\\omega_{si}$.")).toEqual(["E_{gh}", "\\omega_{si}"]);
  });
  it("reads double dollars on their own lines", () => {
    expect(maths("$$\nR = \\delta \\omega\n$$")).toEqual(["R = \\delta \\omega"]);
    expect(maths("$$R_{ghs} = x$$")).toEqual(["R_{ghs} = x"]);
  });
  it("leaves money and shell variables as written", () => {
    expect(maths("It costs $5 to $10 a month.")).toEqual([]);
    expect(maths("Between $5/$10 and more.")).toEqual([]);
    expect(maths("Set $HOME and $PATH first.")).toEqual([]);
    expect(maths("WHERE id = $1 AND asset = ANY($2)")).toEqual([]);
    expect(maths("It costs $0 either way, and `$HOME` isn't read.")).toEqual([]);
    expect(maths("Loud when $L \\ge 50$. It costs $0 either way, and `$HOME` isn't read.")).toEqual(["L \\ge 50"]);
    expect(maths("US$5 and CA$6")).toEqual([]);
  });
  it("still reads what's between the dollars it leaves", () => {
    expect(kinds("It costs $5 with **more** and $10.")).toContain("strong");
  });
  it("isn't read in code", () => {
    expect(maths("`${T}.${T}` and\n\n```\n$a$\n```")).toEqual([]);
  });
});

describe("front matter", () => {
  it("is one block at the top", () => {
    expect(kinds("---\ntitle: One\nstatus: proposed\n---\n\n# One\n").slice(0, 3)).toEqual(["root", "yaml", "heading"]);
  });
  it("is a rule anywhere else", () => {
    expect(kinds("# One\n\n---\n\ntext\n")).toContain("thematicBreak");
  });
});

describe("links", () => {
  it("reads a path against the document's folder", () => {
    expect(resolveLink("bp/accepted/a.md", "../proposed/b.md#summary")).toEqual({ kind: "file", path: "bp/proposed/b.md", hash: "summary" });
    expect(resolveLink("README.md", "domain/sectors.md")).toEqual({ kind: "file", path: "domain/sectors.md", hash: "" });
    expect(resolveLink("a/b/c.md", "./d%20e.md")).toEqual({ kind: "file", path: "a/b/d e.md", hash: "" });
  });
  it("reads a leading slash as the repo's root", () => {
    expect(resolveLink("a/b/c.md", "/docs/x.md")).toEqual({ kind: "file", path: "docs/x.md", hash: "" });
  });
  it("knows a heading, an address, and a path that leaves the repo", () => {
    expect(resolveLink("a.md", "#how-it-lands")).toEqual({ kind: "heading", id: "how-it-lands" });
    expect(resolveLink("a.md", "https://example.com/x")).toEqual({ kind: "external", href: "https://example.com/x" });
    expect(resolveLink("a.md", "mailto:a@example.com").kind).toBe("external");
    expect(resolveLink("a/b.md", "../../other/c.md")).toEqual({ kind: "outside" });
  });
});

describe("slugs", () => {
  it("names headings as GitHub does", () => {
    const slug = slugger();
    expect(slug("1. Summary")).toBe("1-summary");
    expect(slug("How it's reviewed (offline)")).toBe("how-its-reviewed-offline");
    expect(slug("Open questions")).toBe("open-questions");
    expect(slug("Open questions")).toBe("open-questions-1");
    expect(slug("ADR-0009: UI_State")).toBe("adr-0009-ui_state");
  });
});
