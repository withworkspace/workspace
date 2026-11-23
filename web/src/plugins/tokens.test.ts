import { readFileSync } from "node:fs";
import { TOKENS } from "@withworkspace/ui-lib/protocol";
import { describe, expect, it } from "vitest";
import { ownVar, shellStyles, tokenVar } from "./tokens";

// The page ws serves for a plugin's app has ws's tokens in a <style>
// (plugin-shell.css, which the build writes with shellStyles): every token
// the frame protocol promises, under its --ws-* name, before anything of
// the plugin's has run.

const read = (file: string) => readFileSync(new URL(file, import.meta.url), "utf8");
const foundations = read("../foundations.css");
const base = read("./shell.css");
const shell = shellStyles(foundations, base);

const uncommented = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
// The custom properties a stylesheet declares, each once.
const declared = (css: string) => new Set([...uncommented(css).matchAll(/(?:^|[{;\s])(--[\w-]+)\s*:/g)].map((m) => m[1]));

describe("the shell's styles", () => {
  it.each(TOKENS)("define --ws-%s, from a token of ws's own that's always set", (token) => {
    expect(shell).toContain(`${tokenVar(token)}:var(${ownVar(token)})`);
    // ws's own is defined in the light theme's block, which the dark
    // theme's only overrides: so it has a value in both.
    const light = uncommented(foundations).slice(0, uncommented(foundations).indexOf(':root[data-theme="dark"]'));
    expect(declared(light), ownVar(token)).toContain(ownVar(token));
  });

  it("define nothing as --ws-* that the protocol doesn't promise", () => {
    const names = [...declared(shell)].filter((n) => n.startsWith("--ws-")).sort();
    expect(names).toEqual(TOKENS.map(tokenVar).sort());
  });

  it("have every one of ws's foundations, for both themes", () => {
    for (const name of declared(foundations)) expect(declared(shell), name).toContain(name);
    expect(shell).toContain(':root[data-theme="dark"] {');
    expect(shell).not.toContain("/*");
  });

  it("paint the page in ws's colours before the app has loaded", () => {
    expect(shell).toMatch(/html,\s*body \{[^}]*background: var\(--ws-bg\);[^}]*color: var\(--ws-fg\);/);
  });

  it("leave the page see-through in ws's frame, so ws's backdrop shows behind it", () => {
    expect(shell).toMatch(/html\[data-framed\],\s*html\[data-framed\] body \{\s*background: transparent;?\s*\}/);
  });
});

// The foundations are in a plugin's page too: a rule there, of anything
// but :root, would style the plugin's app.
describe("the foundations", () => {
  it("are custom properties on :root and nothing else", () => {
    const selectors = [...uncommented(foundations).matchAll(/(?:^|\})\s*([^{}]+?)\s*\{/g)].map((m) => m[1]);
    expect(selectors).toEqual([":root", ':root[data-theme="dark"]']);
    const properties = [...uncommented(foundations).matchAll(/(?:^|[{;])\s*([\w-]+)\s*:/g)].map((m) => m[1]);
    expect(properties.filter((p) => !p.startsWith("--") && p !== "color-scheme")).toEqual([]);
  });
});

// The shell's own rules are in the plugin's page: they speak of the tokens
// as the app does, by the names the protocol promises, so they're a fair
// example of what an app can rely on.
describe("the shell's own rules", () => {
  it("use only the tokens a plugin's app is promised", () => {
    // --ws-ref-accent isn't a token: it's a ref's own colour, which
    // whoever drew the ref put on its link. --fast and --ease are ws's own
    // names, for the backdrop's fade alone: it has to move as ws's half of
    // the dim does (styles.css: .frame-dim), which is timed by them.
    const own = ["--ws-ref-accent", "--fast", "--ease"];
    const used = [...uncommented(base).matchAll(/var\((--[\w-]+)/g)].map((m) => m[1]).filter((name) => !own.includes(name));
    expect(used.length).toBeGreaterThan(0);
    for (const name of used) expect(TOKENS.map(tokenVar) as string[], name).toContain(name);
  });

  it("style a ref as the frame protocol says one is drawn, and nothing else of the app's", () => {
    expect(shell).toMatch(/a\.ws-ref\[data-ws-ref\] \{[^}]*text-decoration: underline dotted var\(--ws-faint\);/);
    expect(shell).toMatch(/a\.ws-ref\[data-ws-ref\]\[data-ws-shape="pill"\] \{[^}]*var\(--ws-ref-accent, var\(--ws-accent\)\)/);
    // Every rule of the shell's is the page's own, a message of ws's, a
    // ref's, or the backdrop's: none reaches into what the app renders.
    // (An at-rule and a keyframe's step aren't selectors.)
    const selectors = [...uncommented(base).matchAll(/(?:^|\})\s*([^{}]+?)\s*\{/g)].flatMap((m) => m[1].split(/,\s*(?![^(]*\))/)).filter((s) => !s.startsWith("@") && s !== "from");
    for (const s of selectors) expect(s, s).toMatch(/^(html(\[data-framed\])?( body)?|body|\.ws-shell-message( \w+)?|a\.ws-ref\[data-ws-ref\].*|\.ws-backdrop)$/);
  });

  it("style a backdrop in ws's scrim, fading in as ws's half of the dim does", () => {
    expect(shell).toMatch(/\.ws-backdrop \{[^}]*position: fixed;[^}]*inset: 0;[^}]*background: var\(--ws-scrim\);[^}]*animation: ws-fade var\(--fast\) var\(--ease\);/);
    // ws's half, beside the frame: the same scrim, and the same fade.
    const styles = read("../styles.css");
    expect(read("../chrome/Shell.module.css")).toMatch(/\.dim \{[^}]*background: var\(--scrim\);[^}]*animation: global\(fade\) var\(--fast\) var\(--ease\);/);
    expect(styles).toMatch(/@keyframes fade \{\s*from \{\s*opacity: 0;\s*\}\s*\}/);
    expect(shell).toMatch(/@keyframes ws-fade \{\s*from \{\s*opacity: 0;\s*\}\s*\}/);
  });
});
