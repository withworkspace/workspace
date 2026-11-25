import type { DiffFile } from "../api";
import { highlightLines, languageOf } from "./highlight";
import type { Highlighted } from "./model";

// highlight colours a file's lines: each hunk's sides as wholes, and the
// file at head for lines opened up from gaps. (An artifact's source diff
// is coloured the same.)
export function highlight(f: DiffFile, blob: string[] | undefined): Highlighted {
  const lang = languageOf(f.path);
  const out: Highlighted = { old: new Map(), new: new Map(), shown: [] };
  for (const h of f.hunks) {
    const olds = h.lines.filter((l) => l.k !== "a");
    const news = h.lines.filter((l) => l.k !== "d");
    highlightLines(olds.map((l) => l.t), lang).forEach((html, i) => out.old.set(olds[i].o!, html));
    highlightLines(news.map((l) => l.t), lang).forEach((html, i) => out.new.set(news[i].n!, html));
  }
  if (blob) out.shown = highlightLines(blob, lang);
  return out;
}
