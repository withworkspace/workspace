import remarkParse from "remark-parse";
import { unified } from "unified";
import { describe, expect, it } from "vitest";
import { lineBreaks } from "./plugins";

// lineBreaks: what someone said keeps the lines they said it in.
function kinds(md: string): string[] {
  const tree = unified().use(lineBreaks).runSync(unified().use(remarkParse).parse(md));
  const out: string[] = [];
  const walk = (n: { type: string; value?: string; children?: unknown[] }) => {
    out.push(n.type === "text" ? `text:${n.value}` : n.type);
    (n.children as (typeof n)[] | undefined)?.forEach(walk);
  };
  walk(tree as never);
  return out;
}

describe("lineBreaks", () => {
  it("makes a line break in a paragraph a break", () => {
    expect(kinds("one\ntwo")).toEqual(["root", "paragraph", "text:one", "break", "text:two"]);
  });

  it("finds them inside marks too", () => {
    expect(kinds("**a\nb**")).toEqual(["root", "paragraph", "strong", "text:a", "break", "text:b"]);
  });

  it("leaves code as it is", () => {
    expect(kinds("```\na\nb\n```")).toEqual(["root", "code"]);
    expect(kinds("`a\nb`")).toEqual(["root", "paragraph", "inlineCode"]);
  });
});
