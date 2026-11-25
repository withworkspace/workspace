import { useMemo, useState } from "react";
import type { DiffFile } from "../api";
import { useHighlighter } from "../review/highlight";
import { type Cell, cellHTML, type FileState, fileRows, type Row } from "../review/model";
import { highlight, RowView } from "../review/Review";
import { ArtifactCompareFile } from "./ArtifactCompareFile";

const noop = () => undefined;

// FileDiff is one file's diff between two versions: unified or side by
// side, coloured, with the unchanged lines between the changes there to
// open. A file that isn't text says only that it differs. Its lines are a
// review's (review/Review.tsx's RowView, in the classes its rules go by:
// lines, split or unified, and plain, for a diff nothing's picked in).
export function FileDiff({ file: f, lines, a, b, split, named }: { file: DiffFile; lines: string[]; a: string; b: string; split: boolean; named: boolean }) {
  const [expanded, setExpanded] = useState<FileState["expanded"]>({});
  const ready = useHighlighter();
  const state: FileState = useMemo(() => ({ viewed: false, blob: lines, expanded }), [lines, expanded]);
  const rows: Row[] = useMemo(() => fileRows(0, f, state, split), [f, state, split]);
  // ready is a dependency: once the highlighter loads, colour.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const hl = useMemo(() => highlight(f, lines), [f, lines, ready]);
  // expand opens up to n more of a gap's lines on each side (all of it if
  // n is 0), as a review does.
  const expand = (gap: number, n: number) =>
    setExpanded((x) => {
      const e = x[gap] ?? { top: 0, bottom: 0 };
      const first = gap === 0;
      const last = gap === f.hunks.length;
      return { ...x, [gap]: n === 0 ? { top: 1e9, bottom: 0 } : { top: first ? 0 : e.top + n, bottom: last ? 0 : e.bottom + n } };
    });
  const html = (c: Cell, side: "old" | "new") => cellHTML(hl, c, side);
  return (
    <ArtifactCompareFile path={named ? f.path : undefined} status={f.status} binary={f.binary} deletions={f.deletions} additions={f.additions} a={a} b={b}>
      {!f.binary && (
        <div className={`lines ${split ? "split" : "unified"} plain`}>
          {rows.slice(1).map((r) => (
            <RowView
              key={r.key}
              row={r}
              split={split}
              cursor={false}
              selected={false}
              html={html}
              onRowClick={noop}
              onGutterDown={noop}
              onDragOver={noop}
              onActivate={(r) => r.type === "gap" && expand(r.gap, r.count !== undefined && r.count <= 40 ? 0 : 20)}
            />
          ))}
        </div>
      )}
    </ArtifactCompareFile>
  );
}
