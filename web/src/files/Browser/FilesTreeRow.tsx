import { cx } from "../../ui/cx";
import { DirtyDot } from "../../ui/DirtyDot";
import { FileIcon } from "../../ui/FileIcon";
import { Icon } from "../../ui/Icon";
import { FilesMark } from "./FilesMark";
import css from "./FilesTreeRow.module.css";
import type { FileStatus } from "./words";

type FilesTreeRowProps = {
  name: string;
  path: string;
  depth: number;
  dir?: boolean;
  open?: boolean;
  // The keys are on it.
  cursor?: boolean;
  // It's the file that's open.
  current?: boolean;
  // A folder: how many changed files are in it, marked with a dot (none
  // while only what's changed is shown).
  changed?: number;
  // A file: how it's changed, and whether that's committed.
  status?: FileStatus;
  uncommitted?: boolean;
  onClick: () => void;
};

// FilesTreeRow is a folder or a file in the explorer's tree: a folder's
// chevron, or a file's mark and icon; its name; and its dot.
export function FilesTreeRow({ name, path, depth, dir, open, cursor, current, changed = 0, status, uncommitted, onClick }: FilesTreeRowProps) {
  return (
    <div
      role="treeitem"
      aria-expanded={dir ? open : undefined}
      aria-selected={current}
      data-path={path}
      className={cx(css.row, dir && css.dir, cursor && css.cursor, current && css.current, status === "deleted" && css.deleted)}
      style={{ paddingLeft: 8 + depth * 14 }}
      onClick={onClick}
      title={path}
    >
      {dir ? (
        <>
          <Icon name={open ? "chevron-down" : "chevron-right"} size={12} className={css.chevron} />
          <span className={css.name}>{name}</span>
          {changed > 0 && <span className={css.changed} title={`${changed} changed`} />}
        </>
      ) : (
        <>
          <FilesMark status={status} uncommitted={uncommitted} />
          <FileIcon path={path} className={css.icon} />
          <span className={css.name}>{name}</span>
          {uncommitted && <DirtyDot className={css.dirty} />}
        </>
      )}
    </div>
  );
}
