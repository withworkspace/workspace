import { useMemo, useState } from "react";
import type { TreeFile } from "../../api";
import { Finder, FinderNone } from "../../ui/Finder";
import { fuzzy, marked } from "../../ui/fuzzy";
import { FilesFinderRow } from "./FilesFinderRow";

// FileFinder finds a file by a few of its letters: t or ⌘P.
export function FileFinder({ files, onPick, onClose }: { files: TreeFile[]; onPick: (path: string) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const matches = useMemo(() => {
    if (!q.trim()) {
      // Before you type: what's changed first, then the rest.
      return [...files]
        .sort((a, b) => Number(!!b.status) - Number(!!a.status))
        .slice(0, 60)
        .map((f) => ({ f, at: [] as number[] }));
    }
    const out: { f: TreeFile; at: number[]; score: number }[] = [];
    for (const f of files) {
      const m = fuzzy(f.path, q);
      if (m) out.push({ f, at: m.at, score: m.score + (f.status ? 1.5 : 0) });
    }
    return out.sort((a, b) => b.score - a.score).slice(0, 60);
  }, [files, q]);
  return (
    <Finder aria-label="Find a file" placeholder="Find a file by a few of its letters…" value={q} onChange={setQ} count={matches.length} onPick={(i) => onPick(matches[i].f.path)} onClose={onClose}>
      {matches.map((m, i) => (
        <FilesFinderRow key={m.f.path} index={i} path={m.f.path} runs={marked(m.f.path, m.at)} status={m.f.status} uncommitted={m.f.uncommitted} />
      ))}
      {!matches.length && <FinderNone>No file matches “{q}”.</FinderNone>}
    </Finder>
  );
}
