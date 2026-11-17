import { toHast } from "mdast-util-to-hast";
import remarkParse from "remark-parse";
import { unified } from "unified";
import { describe, expect, it } from "vitest";
import type { Known } from "./refs";
import { remarkRefs } from "./remark";

const known: Known = {
  repos: new Map([["demo", new Set(["main.go", "config/app.json"])]]),
  root: "/Users/me/ws",
  org: "acme",
  defaultBranch: "main",
  missions: ["fix-token-refresh"],
  pages: [],
  talks: ["w4"],
};

// marked is each ref the reader would draw, as the words it's drawn from
// and what it names.
function marked(md: string, links = true) {
  const tree = unified().use(remarkParse).use(remarkRefs, { known, links }).runSync(unified().use(remarkParse).parse(md));
  const out: [string, string][] = [];
  const text = (n: unknown): string => {
    const x = n as { type: string; value?: string; children?: unknown[] };
    return x.type === "text" ? (x.value ?? "") : (x.children ?? []).map(text).join("");
  };
  const walk = (n: unknown) => {
    const x = n as { type: string; properties?: Record<string, unknown>; children?: unknown[] };
    const ref = x.properties?.dataRef;
    if (typeof ref === "string") out.push([text(x), JSON.parse(ref).kind]);
    x.children?.forEach(walk);
  };
  walk(toHast(tree as never));
  return out;
}

describe("refs in markdown, as it's read", () => {
  it("marks names in the words and in backticks", () => {
    expect(marked("It's in demo/main.go:3, see `demo/config/app.json` and w4, for fix-token-refresh.")).toEqual([
      ["demo/main.go:3", "file"],
      ["demo/config/app.json", "file"],
      ["w4", "talk"],
      ["fix-token-refresh", "mission"],
    ]);
  });

  it("marks a link an agent made to a file on this disk, keeping its words", () => {
    expect(marked("Keycloak: [production auth configuration](/Users/me/ws/repos/demo/.ground/config/app.json:15).")).toEqual([["production auth configuration", "file"]]);
    // A document read where it is follows its own links.
    expect(marked("[the config](/Users/me/ws/repos/demo/.ground/config/app.json) and [main](main.go)", false)).toEqual([]);
  });

  it("reads an address in one of the workspace's repos as what it names, in a document too", () => {
    const md = "See [ADR 26](https://github.com/ACME/demo/blob/main/config/app.json), [PR](https://github.com/acme/demo/pull/3) and [elsewhere](https://github.com/other/demo/blob/main/main.go).";
    expect(marked(md, false)).toEqual([
      ["ADR 26", "file"],
      ["PR", "pr"],
    ]);
  });

  it("reads a repo's name with a path in it after, in backticks, as the file", () => {
    expect(marked("Implements it (demo `config/app.json`), and demo `config/`.")).toEqual([
      ["demo config/app.json", "file"],
      ["demo config/", "dir"],
    ]);
    // A path that isn't in the repo leaves the name as the repo.
    expect(marked("See demo `nope.md`.")).toEqual([["demo", "repo"]]);
  });

  it("leaves code blocks, headings, a link's words and code that's more than a name", () => {
    expect(marked("# demo/main.go\n\n```\ndemo/main.go\n```\n\n[demo/main.go](https://example.com) `cat demo/main.go`")).toEqual([]);
  });
});
