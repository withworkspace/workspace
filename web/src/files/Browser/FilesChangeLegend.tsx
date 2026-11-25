import css from "./FilesChangeLegend.module.css";

// FilesChangeLegend says what the marks beside the line numbers mean: the
// kinds of change there are in the file, each by its mark.
export function FilesChangeLegend({ added, modified, deleted }: { added?: boolean; modified?: boolean; deleted?: boolean }) {
  return (
    <span className={css.legend} title="Marked beside the line numbers">
      {added && (
        <span>
          <i className={css.added} /> added
        </span>
      )}
      {modified && (
        <span>
          <i className={css.modified} /> changed
        </span>
      )}
      {deleted && (
        <span>
          <i className={css.deleted} /> removed after
        </span>
      )}
    </span>
  );
}
