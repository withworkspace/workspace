import css from "./ReviewProgress.module.css";

// ReviewProgress is how many of a review's files you've viewed, as a bar.
export function ReviewProgress({ viewed, of }: { viewed: number; of: number }) {
  return (
    <div className={css.progress} title={`${viewed} of ${of} ${of === 1 ? "file" : "files"} viewed`}>
      <div style={{ width: of ? `${(100 * viewed) / of}%` : 0 }} />
    </div>
  );
}
