import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";
import "./all";
import { allScopes } from "./commands";
import { keysOf } from "./keymap";

const src = join(__dirname, "..");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((n) => {
    const p = join(dir, n);
    if (statSync(p).isDirectory()) return n === "node_modules" ? [] : files(p);
    return /\.tsx?$/.test(n) && !/\.(test|stories)\.tsx?$/.test(n) ? [p] : [];
  });
}
const all = files(src);
const said = (p: string) => readFileSync(p, "utf8");

describe("every scope", () => {
  it("is read by keymap/all.ts, so Settings lists it", () => {
    const index = said(join(src, "keymap/all.ts"));
    const making = all.filter((p) => !p.includes("/keymap/") && /= scope\("/.test(said(p)));
    const missing = making.map((p) => relative(join(src, "keymap"), p).replace(/\.tsx?$/, "")).filter((r) => !index.includes(`"${r}"`));
    expect(missing).toEqual([]);
  });

  it("has an id of its own, and commands with a title each", () => {
    const ids = allScopes().map((s) => s.id);
    expect(ids.length).toBe(new Set(ids).size);
    for (const s of allScopes()) for (const c of s.commands) expect(c.title, c.id).not.toBe("");
  });

  it("has keys written as a press is named", () => {
    // Not as they're said: ⏎, esc, ⌘ and Shift+ are how they're drawn.
    for (const s of allScopes())
      for (const c of s.commands) {
        if (c.fixed) continue;
        for (const k of keysOf(c, {})) expect(k, c.id).not.toMatch(/⏎|⌘|⇧|^esc$|Shift\+|Ctrl\+|mod\+[A-Z]$/);
      }
  });
});

describe("the old way of binding keys", () => {
  it("is gone: every key's a command", () => {
    const old = all.filter((p) => /\buse(Keys|Shortcut)\(/.test(said(p)));
    expect(old.map((p) => relative(src, p))).toEqual([]);
  });
});
