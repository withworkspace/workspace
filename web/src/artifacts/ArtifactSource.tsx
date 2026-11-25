import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ArtifactSource.module.css";

// ArtifactSource is a version's source, in its stage: the file being read
// (its children) and, for a version with more than its page, the list of
// its files beside it (files: an ArtifactSourceFiles). On a narrow window
// the list is over the file.
export function ArtifactSource({ files, children }: { files?: ReactNode; children: ReactNode }) {
  return (
    <div className={cx(css.view, files ? css.many : undefined)}>
      {files}
      {children}
    </div>
  );
}
