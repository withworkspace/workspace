import { readdirSync, readFileSync, statSync } from "node:fs";
import { basename, dirname, join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

// A CSS module is typed as any name to a string (vite/client), so a class
// misspelt where it's read (css.rwo) is undefined, with nothing said. This
// reads each module and every file that imports it: each class read is one
// the module has, and each class the module has is read somewhere.

const src = dirname(new URL(import.meta.url).pathname);

function files(dir: string, keep: (f: string) => boolean, out: string[] = []): string[] {
  for (const f of readdirSync(dir)) {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) files(p, keep, out);
    else if (keep(f)) out.push(p);
  }
  return out;
}

// classesOf is the classes a module defines: those under :global() are
// the page's own, not the module's.
function classesOf(css: string): Set<string> {
  const bare = css
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/url\([^)]*\)/g, "")
    .replace(/:global\([^)]*\)/g, "");
  return new Set([...bare.matchAll(/\.(-?[_a-zA-Z][\w-]*)/g)].map((m) => m[1]!));
}

// readsOf is the classes a file reads from the module it imports as name.
function readsOf(code: string, name: string): Set<string> {
  const dot = [...code.matchAll(new RegExp(`\\b${name}\\.([A-Za-z_$][\\w$]*)`, "g"))].map((m) => m[1]!);
  const index = [...code.matchAll(new RegExp(`\\b${name}\\[["'\`]([^"'\`]+)["'\`]\\]`, "g"))].map((m) => m[1]!);
  return new Set([...dot, ...index]);
}

const modules = files(src, (f) => f.endsWith(".module.css"));
const code = files(src, (f) => /\.tsx?$/.test(f) && !f.endsWith(".test.ts")).map((p) => ({ p, text: readFileSync(p, "utf8") }));

describe("CSS modules", () => {
  it("are found", () => {
    expect(modules.length).toBeGreaterThan(0);
  });

  for (const m of modules) {
    const name = relative(src, m);
    describe(name, () => {
      const defined = classesOf(readFileSync(m, "utf8"));
      const importers = code.flatMap(({ p, text }) =>
        [...text.matchAll(/import\s+(\w+)\s+from\s+["']([^"']+\.module\.css)["']/g)]
          .filter((i) => resolve(dirname(p), i[2]!) === m)
          .map((i) => ({ p, reads: readsOf(text, i[1]!) })),
      );

      it("names each class so it reads as css.name", () => {
        expect([...defined].filter((c) => !/^[a-z][a-zA-Z0-9]*$/.test(c))).toEqual([]);
      });

      it("is imported", () => {
        expect(importers.map((i) => basename(i.p))).not.toEqual([]);
      });

      it("has every class it's read for", () => {
        const missing = importers.flatMap((i) => [...i.reads].filter((c) => !defined.has(c)).map((c) => `${relative(src, i.p)}: ${c}`));
        expect(missing).toEqual([]);
      });

      it("has no class nothing reads", () => {
        const read = new Set(importers.flatMap((i) => [...i.reads]));
        expect([...defined].filter((c) => !read.has(c))).toEqual([]);
      });
    });
  }
});

describe("classesOf and readsOf", () => {
  it("leave out :global() and comments, and read both ways of naming", () => {
    expect([...classesOf("/* .no */ .a :global(.b) .c:hover { background: url(x.png) } @media (width < 2.5em) { .d {} }")]).toEqual(["a", "c", "d"]);
    expect([...readsOf('cx(css.a, open && css.b, css["c"]); other.d', "css")]).toEqual(["a", "b", "c"]);
  });
});
