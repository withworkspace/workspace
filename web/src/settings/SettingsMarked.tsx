import type { ReactNode } from "react";
import css from "./SettingsMarked.module.css";

// SettingsMarked is text with words marked in it: what Find has typed,
// each lower case, where a row says it.
export function SettingsMarked({ text, words }: { text: string; words: string[] }) {
  if (words.length === 0) return <>{text}</>;
  const lower = text.toLowerCase();
  const parts: ReactNode[] = [];
  let at = 0;
  while (at < text.length) {
    // The nearest of the words from here, the longest of those that start there.
    let from = -1;
    let len = 0;
    for (const w of words) {
      const i = lower.indexOf(w, at);
      if (i >= 0 && (from < 0 || i < from || (i === from && w.length > len))) [from, len] = [i, w.length];
    }
    if (from < 0) break;
    if (from > at) parts.push(text.slice(at, from));
    parts.push(
      <mark key={from} className={css.mark}>
        {text.slice(from, from + len)}
      </mark>,
    );
    at = from + len;
  }
  parts.push(text.slice(at));
  return <>{parts}</>;
}
