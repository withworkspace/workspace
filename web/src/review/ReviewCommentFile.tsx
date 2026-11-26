import css from "./ReviewCommentFile.module.css";

// ReviewCommentFile heads a file's comments in a review's list: its
// folder quiet, its name, how many. "" is the whole change.
export function ReviewCommentFile({ path, count }: { path: string; count: number }) {
  const cut = path.lastIndexOf("/") + 1;
  return (
    <div className={css.file}>
      {path ? (
        <>
          <span className={css.dir}>{path.slice(0, cut)}</span>
          <span className={css.name}>{path.slice(cut)}</span>
        </>
      ) : (
        <span className={css.name}>The whole change</span>
      )}
      <span className={css.count}>{count}</span>
    </div>
  );
}
