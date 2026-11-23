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
