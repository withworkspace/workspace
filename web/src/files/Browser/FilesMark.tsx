import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./FilesMark.module.css";
import { type FileStatus, statusIcon, statusWords } from "./words";

const tones: Partial<Record<FileStatus, string>> = { added: css.added, deleted: css.deleted, renamed: css.renamed, copied: css.copied };

// FilesMark is how a file's changed since the branch point, before its
// name: added, removed, renamed, or a dot for changed. One that hasn't is
// a blank of the same size, so names line up.
export function FilesMark({ status, uncommitted }: { status?: FileStatus; uncommitted?: boolean }) {
  if (!status) return <span className={css.mark} aria-hidden />;
  return (
    <span className={cx(css.mark, css.status, tones[status])} title={`${statusWords[status] ?? status} since the branch point${uncommitted ? ", not committed" : ""}`}>
      {statusIcon[status] ? <Icon name={statusIcon[status]} size={10} /> : <span className={css.dot} />}
    </span>
  );
}
