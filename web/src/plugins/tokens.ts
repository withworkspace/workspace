import { TOKENS, type Token, type TokenVar } from "@withworkspace/ui-lib/protocol";

// The design tokens a plugin's app is given, as --ws-<name>: ws's own
// (foundations.css), under the names the frame protocol promises a page.
// Which there are is the protocol's to say (TOKENS); what each is called
// in ws is said here, once, for the two places that give them: the shell's
// <style>, built from this (shellStyles), and the values ws sends when the
// theme changes (theme.ts). Nothing here touches the DOM: the build
// imports it.

// ws's own name for a token, where it isn't the protocol's.
const own: Partial<Record<Token, string>> = { danger: "red", warn: "amber", ok: "green" };

// ownVar is the custom property of ws's that a token is: --red, for danger.
export const ownVar = (token: Token): string => `--${own[token] ?? token}`;

// tokenVar is a token as a plugin's app knows it: --ws-danger.
export const tokenVar = (token: Token): TokenVar => `--ws-${token}`;

// shellStyles is what the shell's <style> holds: ws's foundations, then
// each token under its --ws-* name, then the shell's own few rules (base).
// The names are defined from ws's own, on the same element, so each
// follows the theme and the accent as ws's own does, with no script.
export function shellStyles(foundations: string, base: string): string {
  const names = TOKENS.map((token) => `${tokenVar(token)}:var(${ownVar(token)})`).join(";");
  return [compact(foundations), `:root{${names}}`, compact(base)].join("\n") + "\n";
}

// compact is css without its comments and the space around its lines:
// it's written into every shell.
function compact(css: string): string {
  return css
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .join("\n");
}
