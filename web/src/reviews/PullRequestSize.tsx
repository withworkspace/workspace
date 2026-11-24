import css from "./PullRequestSize.module.css";

type PullRequestSizeProps = {
  additions: number;
  deletions: number;
  // How many files it changes, where that's known.
  files?: number;
};

// PullRequestSize is how much a pull request changes: lines added, lines
// taken away, and across how many files.
export function PullRequestSize({ additions, deletions, files }: PullRequestSizeProps) {
  return (
    <span className={css.size}>
      <span className={css.plus}>+{additions.toLocaleString()}</span>
      <span className={css.minus}>−{deletions.toLocaleString()}</span>
      {files !== undefined && (
        <span className={css.files}>
          {files} {files === 1 ? "file" : "files"}
        </span>
      )}
    </span>
  );
}
