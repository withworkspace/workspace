import { useMemo, useState } from "react";
import { Finder, FinderNone, FinderRow } from "../ui/Finder";
import { fuzzy } from "../ui/fuzzy";

// ReviewFileJump jumps to one of a review's files by a few letters of its
// path: the best fifty, in the order they match.
export function ReviewFileJump({ paths, onPick, onClose }: { paths: string[]; onPick: (i: number) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const matches = useMemo(() => {
    return paths
      .flatMap((path, i) => {
        const m = fuzzy(path, q);
        return m ? [{ path, i, score: m.score }] : [];
      })
      .sort((a, b) => b.score - a.score || a.i - b.i)
      .slice(0, 50);
  }, [paths, q]);
  return (
    <Finder aria-label="Jump to a file" placeholder="Jump to a file" value={q} onChange={setQ} count={matches.length} onPick={(i) => onPick(matches[i].i)} onClose={onClose}>
      {matches.map((m, i) => (
        <FinderRow key={m.path} index={i}>
          <span className="mono">{m.path}</span>
        </FinderRow>
      ))}
      {!matches.length && <FinderNone>No file matches “{q}”.</FinderNone>}
    </Finder>
  );
}
