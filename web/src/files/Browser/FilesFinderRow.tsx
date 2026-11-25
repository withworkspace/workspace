import { cx } from "../../ui/cx";
import { DirtyDot } from "../../ui/DirtyDot";
import { FileIcon } from "../../ui/FileIcon";
import { FinderRow } from "../../ui/Finder";
import { FilesMark } from "./FilesMark";
import css from "./FilesFinderRow.module.css";
import type { FileStatus } from "./words";

// FilesFinderRow is a file found by a few of its letters (in a Finder):
// its mark and icon, then its path, what was typed marked in it and its
// own name standing out.
export function FilesFinderRow({ index, path, runs, status, uncommitted }: { index: number; path: string; runs: { text: string; hit: boolean }[]; status?: FileStatus; uncommitted?: boolean }) {
  const cut = path.lastIndexOf("/") + 1;
  let pos = 0;
  return (
    <FinderRow index={index} className={css.row}>
      <FilesMark status={status} uncommitted={uncommitted} />
      <FileIcon path={path} />
      <span className={css.path}>
        {runs.map((r, j) => {
          const start = pos;
          pos += r.text.length;
          return (
            <span key={j} className={cx(r.hit && css.hit, start >= cut && css.name)}>
              {r.text}
            </span>
          );
        })}
      </span>
      {uncommitted && <DirtyDot />}
    </FinderRow>
  );
}
