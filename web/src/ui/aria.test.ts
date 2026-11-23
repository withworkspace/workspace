import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { describe, expect, it } from "vitest";

// react-aria-components is the library's to use: a page takes ws's part
// (Button, Select…), which is react-aria's in ws's look, with ws's
// defaults. So only web/src/ui imports it. The pages below still did when
// this was written, each until the part it needs is in the library: the
// list only gets shorter, and a page that no longer imports it is taken
// off.
const still = ["shell/Arrange.tsx"];

const src = join(dirname(new URL(import.meta.url).pathname), "..");

function files(dir: string, out: string[] = []): string[] {
  for (const f of readdirSync(dir)) {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) files(p, out);
    else if (/\.tsx?$/.test(f) && !/\.test\.ts$/.test(f)) out.push(p);
  }
  return out;
}

const importers = files(src)
  .filter((p) => /from\s+["']react-aria(-components)?(\/[^"']*)?["']/.test(readFileSync(p, "utf8")))
  .map((p) => relative(src, p))
  .filter((p) => !p.startsWith("ui/"))
  .sort();

describe("react-aria-components", () => {
  it("is imported by the library alone, and the pages still to move", () => {
    expect(importers.filter((p) => !still.includes(p))).toEqual([]);
  });

  it("has no page listed that's moved", () => {
    expect(still.filter((p) => !importers.includes(p))).toEqual([]);
  });
});
