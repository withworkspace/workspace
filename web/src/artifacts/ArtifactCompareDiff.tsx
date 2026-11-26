import type { ReactNode } from "react";
import css from "./ArtifactCompareDiff.module.css";
import { Text } from "../ui/Text";

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
    <Text as="div" dim className={css.count}>
      {files} {files === 1 ? "file differs" : "files differ"} between {a} and {b}
    </Text>
  );
}
