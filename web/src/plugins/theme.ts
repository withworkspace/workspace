import { TOKENS, type V1 } from "@withworkspace/ui-lib/protocol";
import { ownVar, tokenVar } from "./tokens";

// Theme is ws's theme as a plugin's app is given it. It's protocol 1's
// shape, which is ws's own for it too: a later version that wants another
// makes it from this.
export type Theme = V1.Theme;

// theme is ws's theme as a plugin's app takes it: light or dark, and each
// token's value now, read from the page as it's drawn. It's what the
// shell's <style> already gave the app (the same foundations, the same
// theme and accent on <html>), so applying it changes nothing to see: it's
// sent so the app has the values, and again whenever they change.
export function theme(): Theme {
  const root = document.documentElement;
  const style = getComputedStyle(root);
  // Every token the protocol promises is one of ws's own, always set
  // (tokens.test.ts): so each has a value, and this is the whole of Vars.
  const vars = Object.fromEntries(TOKENS.map((token) => [tokenVar(token), style.getPropertyValue(ownVar(token)).trim()])) as V1.Vars;
  const scheme = root.dataset.theme === "dark" || (root.dataset.theme === undefined && style.colorScheme.includes("dark")) ? "dark" : "light";
  return { scheme, vars };
}

// onThemeChange runs fn when the theme or the accent changes: both are on
// <html>, as data-theme and as its style. It returns what stops it.
export function onThemeChange(fn: () => void): () => void {
  const watch = new MutationObserver(fn);
  watch.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "style"] });
  return () => watch.disconnect();
}
