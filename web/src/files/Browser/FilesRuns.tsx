import type { ReactNode } from "react";
import css from "./FilesRuns.module.css";

// FilesRuns is a ground's or capsule's Runs tab, laid out as its Files
// are: the list of its runs where the tree is (list), the one picked where
// the file is (panel). With no room for both, one at a time.
export function FilesRuns({ list, panel }: { list?: ReactNode; panel?: ReactNode }) {
  return (
    <div className={css.runs}>
      {list && (
        <aside className={css.list} aria-label="Runs">
          {list}
        </aside>
      )}
      {panel}
    </div>
  );
}
