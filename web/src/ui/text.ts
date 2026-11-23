// Small things done to text that's shown.

// clip cuts s to n characters at most, its last an ellipsis where it's cut,
// with no space left before it.
export function clip(s: string, n: number): string {
  return s.length > n ? `${s.slice(0, n - 1).trimEnd()}…` : s;
}

// capital is s with a capital first letter.
export function capital(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

// humanError makes a server's error a sentence: capitalised, ending in a
// full stop.
export function humanError(e: string): string {
  const s = e.trim().replace(/^error:\s*/i, "");
  if (!s) return "Something went wrong.";
  const t = capital(s);
  return /[.!?)]$/.test(t) ? t : t + ".";
}

// flatten folds markdown's block syntax into one paragraph.
export function flatten(md: string): string {
  const out: string[] = [];
  let fenced = false;
  for (const raw of md.split("\n")) {
    const line = raw.trim();
    if (/^(```|~~~)/.test(line)) {
      fenced = !fenced;
      continue;
    }
    if (!line) continue;
    if (fenced) {
      out.push("`" + line.replace(/`/g, "") + "`");
      continue;
    }
    if (/^\[\^[^\]]+\]:/.test(line)) continue; // a footnote's definition
    if (/^\|?\s*:?-{3,}/.test(line)) continue; // a table's rule
    const bullet = /^([-*+]|\d+[.)])\s+/.test(line);
    const text = line
      .replace(/^#{1,6}\s+/, "")
      .replace(/^>\s?/, "")
      .replace(/^([-*+]|\d+[.)])\s+(\[[ xX]\]\s+)?/, "");
    out.push(bullet ? `• ${text}` : text);
  }
  return out
    .join(" ")
    .replace(/```[a-z]*\s*([^`]+?)\s*```/g, "`$1`") // a fence written inline
    .replace(/\[\^[^\]]+\]:\s*/g, "") // a footnote's definition, inline
    .replace(/``\s*([^`]+?)\s*``/g, "`$1`");
}

// plain is markdown with no markup at all, in a line: for a title
// attribute or a page's title, where Inline (Inline.tsx) can't be.
export const plain = (md: string) =>
  flatten(md)
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\*\*([^*]+)\*\*|__([^_]+)__/g, "$1$2")
    .replace(/\*([^*\s][^*]*)\*/g, "$1")
    .replace(/\[\^[^\]]+\]/g, "")
    .replace(/\[([^\]]+)\]\([^)\s]+\)/g, "$1");
