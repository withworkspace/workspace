import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ArtifactCompareBody.module.css";

// ArtifactCompareBody is what's under a comparison's head: two sides, each
// a head (an ArtifactCompareSide) over its frame, in two columns; or their
// source diff (an ArtifactCompareDiff), which spans both under the heads.
// one is a narrow window's: one side at a time, in a column.
export function ArtifactCompareBody({ one, children }: { one?: boolean; children: ReactNode }) {
  return <div className={cx(css.body, one && css.one)}>{children}</div>;
}
