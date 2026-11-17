import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// The accent is one colour, --accent-pick (the server puts [ui] accent on
// <html> as it), and every accent token in foundations.css is worked out from
// it, in both themes: none is a colour written out by hand. See
// workspace-records:docs/customisable-ui-accent.md.

const css = readFileSync(new URL("./foundations.css", import.meta.url), "utf8");

// block is the first rule for selector, as written: its declarations by
// custom property.
function block(selector: string): Map<string, string> {
  const start = css.indexOf(`\n${selector} {\n`);
  if (start < 0) throw new Error(`no ${selector} rule`);
  const body = css.slice(start, css.indexOf("\n}\n", start));
  const decls = new Map<string, string>();
  for (const m of body.replace(/\/\*[\s\S]*?\*\//g, "").matchAll(/(--[\w-]+)\s*:\s*([^;]*);/g)) decls.set(m[1], m[2].trim());
  return decls;
}

const light = block(":root");
const dark = block(':root[data-theme="dark"]');
const themes = { light, dark: new Map([...light, ...dark]) };

const derived = ["--accent", "--accent-soft", "--accent-line", "--primary", "--primary-hover", "--on-primary", "--brand-gradient", "--horizon-1", "--horizon-2", "--horizon-3"];

// A colour written out: a hex, or a colour function given its channels
// (rgb(51 102 153), oklch(0.5 0.1 300)), rather than one from another.
const literal = /#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(\s*(?!from\b)[\d.]/i;

// from is whether name, in a theme, comes from --accent-pick: through the
// tokens its value names, with no colour written out on the way.
function from(vars: Map<string, string>, name: string, seen: string[] = []): boolean {
  if (name === "--accent-pick") return true;
  const value = vars.get(name);
  if (value === undefined || seen.includes(name) || literal.test(value)) return false;
  const named = [...value.matchAll(/var\((--[\w-]+)/g)].map((m) => m[1]);
  return named.length > 0 && named.every((n) => from(vars, n, [...seen, name]));
}

describe("the accent", () => {
  it("is picked once, as the default, on :root", () => {
    expect(light.get("--accent-pick")).toMatch(/^#[0-9a-f]{6}$/);
    expect(dark.has("--accent-pick")).toBe(false);
  });

  for (const [theme, vars] of Object.entries(themes)) {
    it.each(derived)(`%s is worked out from --accent-pick in the ${theme} theme`, (name) => {
      expect(vars.get(name), name).toBeDefined();
      expect(from(vars, name), `${name}: ${vars.get(name)}`).toBe(true);
    });
  }

  it("is adjusted for each theme: lightness held down on the light page, lifted on the dark", () => {
    expect(light.get("--brand-1")).toMatch(/^oklch\(from var\(--accent-pick\) min\(l, [\d.]+\)/);
    expect(light.get("--accent")).toBe("var(--brand-1)");
    expect(light.get("--accent-lifted")).toMatch(/^oklch\(from var\(--accent-pick\) max\(l, [\d.]+\)/);
    expect(dark.get("--accent")).toBe("var(--accent-lifted)");
    expect(dark.get("--primary-lift")).toMatch(/^color\(from oklch\(from var\(--accent-pick\) max\(l, [\d.]+\) c h\) srgb-linear/);
    expect(dark.get("--primary")).toMatch(/^color\(from var\(--primary-lift\) xyz-d65/);
  });

  it("labels the button by contrast with it, in either theme", () => {
    const label = light.get("--on-primary") ?? "";
    expect(label).toMatch(/^color\(\s*from var\(--primary\) xyz-d65/);
    expect(label).toMatch(/\(0\.1925 - y\)/);
    expect(dark.has("--on-primary")).toBe(false);
    const hovered = light.get("--on-primary-hover") ?? "";
    expect(hovered).toMatch(/^color\(\s*from var\(--primary-hover\) xyz-d65/);
    expect(hovered.replace("--primary-hover", "--primary")).toBe(label);
  });

  it("finds a colour written out by hand", () => {
    const vars = new Map([...themes.light, ["--accent", "#336699"], ["--accent-soft", "rgb(51 102 153 / 0.1)"], ["--primary-hover", "oklch(0.45 0.12 300)"]]);
    expect(from(vars, "--accent")).toBe(false);
    expect(from(vars, "--accent-soft")).toBe(false);
    expect(from(vars, "--primary-hover")).toBe(false);
    expect(from(vars, "--primary")).toBe(false);
  });
});

// The default violet is written once: as --accent-pick's default, on :root
// in foundations.css. Anywhere else in ws's code (the web UI's sources and
// files, and the Go that serves pages) a copy of it, as hex or as rgb,
// is a colour that won't follow the pick. This file doesn't name it: it
// reads it from that one definition.
describe("the default violet", () => {
  const violet = light.get("--accent-pick") ?? "";
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(violet.slice(i, i + 2), 16));
  const forms = [new RegExp(violet, "i"), new RegExp(`\\b${r}[\\s,]+${g}[\\s,]+${b}\\b`)];

  // copies are the lines of text that have the violet in either form.
  const copies = (text: string) =>
    text
      .split("\n")
      .map((line, i) => ({ line: i + 1, text: line.trim() }))
      .filter((l) => forms.some((f) => f.test(l.text)));

  it("is found as hex in either case, with alpha, and as rgb with spaces or commas", () => {
    for (const text of [`color: ${violet.toUpperCase()};`, `fill="${violet}"`, `${violet}80`, `rgb(${r} ${g} ${b} / 0.1)`, `rgba(${r}, ${g}, ${b}, 0.4)`, `rgb(${r},${g},${b})`]) {
      expect(copies(text), text).toHaveLength(1);
    }
    for (const text of [`${violet.slice(0, -1)}d`, `rgb(${r} ${g} ${b + 1})`, `${r}${g}${b}`]) {
      expect(copies(text), text).toHaveLength(0);
    }
  });

  it("is written only as --accent-pick's default, in web/, internal/ and cmd/", () => {
    expect(violet).toMatch(/^#[0-9a-f]{6}$/);
    const repo = fileURLToPath(new URL("../../", import.meta.url));
    const files = execFileSync("git", ["ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "web", "internal", "cmd"], { cwd: repo, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(files).toContain("web/src/foundations.css");
    const found: string[] = [];
    for (const f of files) {
      let data: Buffer;
      try {
        data = readFileSync(repo + f);
      } catch {
        continue; // deleted, not yet staged
      }
      if (data.subarray(0, 8000).includes(0)) continue; // not text
      for (const c of copies(data.toString("utf8"))) found.push(`${f}:${c.line}: ${c.text}`);
    }
    expect(found).toEqual([expect.stringMatching(/^web\/src\/foundations\.css:\d+: --accent-pick: #[0-9a-f]{6};$/)]);
  });
});
