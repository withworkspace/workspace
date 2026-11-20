import { useEffect, useRef, useState } from "react";
import { useKeys } from "../keys";
import { type CodeEnd, type CodeHit, codeNote, codeRematch, codeStream } from "../shell/find";
import { CodeText, CodeWhere } from "../shell/CodeHitParts";
import { FileIcon } from "../ui/FileIcon";
import { Icon } from "../ui/Icon";
import { humanError } from "../ui/kit";

// CodeFinder is the explorer's own search, in a capsule: what its files
// say, all of them (unchanged ones too: here you've chosen to look), as
// ws rg finds it. ⌘⇧F opens it, nearer than Find's. A hit opens its file
// at its line.

// wait is how long it waits for typing to pause before it asks.
const wait = 150;

export function CodeFinder({ repo, worktree, onPick, onClose }: { repo: string; worktree: string; onPick: (path: string, line: number) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const [got, setGot] = useState<{ q: string; hits: CodeHit[]; end?: CodeEnd; done: boolean }>({ q: "", hits: [], done: true });
  const list = useRef<HTMLUListElement>(null);
  useKeys({ Escape: onClose }, { modal: true });

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
  const at = Math.min(sel, Math.max(0, hits.length - 1));
  const move = (d: number) => {
    const n = Math.max(0, Math.min(at + d, hits.length - 1));
    setSel(n);
    list.current?.children[n]?.scrollIntoView({ block: "nearest" });
  };
  const note = codeNote(got.end);
  return (
    <div className="overlay palette-overlay" onMouseDown={onClose}>
      <div className="palette code-finder" role="dialog" aria-modal="true" aria-label="Search the files" onMouseDown={(e) => e.stopPropagation()}>
        <div className="palette-input">
          <Icon name="search" size={16} />
          <input
            autoFocus
            value={q}
            placeholder={`Text in ${repo}/${worktree}'s files…`}
            aria-label="Search the files"
            role="combobox"
            aria-expanded="true"
            aria-controls="code-finder-list"
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => {
              setQ(e.target.value);
              setSel(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")) {
                e.preventDefault();
                move(1);
              } else if (e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")) {
                e.preventDefault();
                move(-1);
              } else if (e.key === "Enter" && hits[at]) {
                e.preventDefault();
                onPick(hits[at].path, hits[at].line);
              }
            }}
          />
          {!got.done && <Icon name="spinner" size={14} className="find-busy" />}
        </div>
        <ul className="palette-list find-list" id="code-finder-list" ref={list} role="listbox">
          {hits.map((h, i) => (
            <li
              key={`${h.path}:${h.line}:${i}`}
              role="option"
              aria-selected={i === at}
              className={`find-row find-code${i === at ? " on" : ""}`}
              onMouseMove={() => i !== at && setSel(i)}
              onClick={() => onPick(h.path, h.line)}
            >
              <FileIcon path={h.path} />
              <CodeWhere h={h} />
              <CodeText h={h} />
            </li>
          ))}
          {!hits.length && got.done && <li className="palette-none">{want ? `No file here says “${want}”.` : "Type text to find in every file of the capsule."}</li>}
        </ul>
        {note && <p className={`find-note${got.end?.capped ? " find-capped" : ""}`}>{note}</p>}
      </div>
    </div>
  );
}
