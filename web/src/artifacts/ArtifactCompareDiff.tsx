import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ArtifactCompareDiff.module.css";

// ArtifactCompareDiff is two versions' source diff, under both sides'
// heads: each file that differs (an ArtifactCompareFile), scrolling by
// itself.
export function ArtifactCompareDiff({ children }: { children: ReactNode }) {
  return <div className={css.diff}>{children}</div>;
}

// ArtifactCompareDiffCount says how many files differ, over them, where
// there's more than the page to tell apart.
export function ArtifactCompareDiffCount({ files, a, b }: { files: number; a: string; b: string }) {
  return (
    <div className={cx(css.count, "dim")}>
      {files} {files === 1 ? "file differs" : "files differ"} between {a} and {b}
    </div>
  );
}
