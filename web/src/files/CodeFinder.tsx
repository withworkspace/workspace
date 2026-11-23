import { useEffect, useState } from "react";
import { type CodeEnd, type CodeHit, codeNote, codeRematch, codeStream } from "../shell/find";
import { CodeText, CodeWhere } from "../shell/CodeHitParts";
import { FileIcon } from "../ui/FileIcon";
import { humanError } from "../ui/kit";
import { Finder, FinderNone, FinderRow } from "../ui/Finder";

// CodeFinder is the explorer's own search, in a capsule: what its files
// say, all of them (unchanged ones too: here you've chosen to look), as
// ws rg finds it. ⌘⇧F opens it, nearer than Find's. A hit opens its file
// at its line.

// wait is how long it waits for typing to pause before it asks.
const wait = 150;

export function CodeFinder({ repo, worktree, onPick, onClose }: { repo: string; worktree: string; onPick: (path: string, line: number) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [got, setGot] = useState<{ q: string; hits: CodeHit[]; end?: CodeEnd; done: boolean }>({ q: "", hits: [], done: true });

  const want = q.trim();
  useEffect(() => {
    if (!want) {
      setGot({ q: "", hits: [], done: true });
      return;
    }
    const drop = new AbortController();
    setGot((was) => ({ q: want, hits: codeRematch(was.hits, want), done: false }));
    const t = setTimeout(() => {
      const hits: CodeHit[] = [];
      let frame = 0;
      codeStream([want], { repo, worktree }, drop.signal, (h) => {
        hits.push(h);
        if (!frame)
          frame = requestAnimationFrame(() => {
            frame = 0;
            if (!drop.signal.aborted) setGot({ q: want, hits: [...hits], done: false });
          });
      }).then(
        (end) => {
          cancelAnimationFrame(frame);
          if (!drop.signal.aborted) setGot({ q: want, hits, end, done: true });
        },
        (e: Error) => {
          cancelAnimationFrame(frame);
          if (!drop.signal.aborted) setGot({ q: want, hits, end: { matches: hits.length, bytes: 0, problem: humanError(e.message) }, done: true });
        },
      );
    }, wait);
    return () => {
      clearTimeout(t);
      drop.abort();
    };
  }, [want, repo, worktree]);

  const hits = got.hits;
  const note = codeNote(got.end);
  return (
    <Finder
      aria-label="Search the files"
      placeholder={`Text in ${repo}/${worktree}'s files…`}
      value={q}
      onChange={setQ}
      count={hits.length}
      onPick={(i) => onPick(hits[i].path, hits[i].line)}
      onClose={onClose}
      busy={!got.done}
      className="code-finder"
      listClassName="find-list"
      after={note && <p className={`find-note${got.end?.capped ? " find-capped" : ""}`}>{note}</p>}
    >
      {hits.map((h, i) => (
        <FinderRow key={`${h.path}:${h.line}:${i}`} index={i} className="find-row find-code">
              <FileIcon path={h.path} />
              <CodeWhere h={h} />
              <CodeText h={h} />
        </FinderRow>
      ))}
      {!hits.length && got.done && <FinderNone className="palette-none">{want ? `No file here says “${want}”.` : "Type text to find in every file of the capsule."}</FinderNone>}
    </Finder>
  );
}
