import { marked } from "../../ui/fuzzy";

// TitlePart is a run of a row's title: letters that matched what's typed
// (hit), and, in a path, its name after its folders (name).
export type TitlePart = { text: string; hit?: boolean; name?: boolean };

// titleParts cuts a row's title into runs: the letters that matched (at,
// their places in it) apart from the rest. A path's last piece (after its
// last / or :) is its name: a run that crosses into it is split there, so
// the name stands out whole.
export function titleParts(title: string, at: number[], path?: boolean): TitlePart[] {
  const cut = path ? Math.max(title.lastIndexOf("/"), title.lastIndexOf(":")) + 1 : 0;
  let pos = 0;
  return marked(title, at).flatMap((r): TitlePart[] => {
    const from = pos;
    pos += r.text.length;
    if (!path) return [{ text: r.text, hit: r.hit }];
    if (from < cut && pos > cut) {
      return [
        { text: r.text.slice(0, cut - from), hit: r.hit },
        { text: r.text.slice(cut - from), hit: r.hit, name: true },
      ];
    }
    return [{ text: r.text, hit: r.hit, name: from >= cut }];
  });
}
