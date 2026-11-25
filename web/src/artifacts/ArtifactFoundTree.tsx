import type { ReactNode } from "react";
import { Rows } from "../ui/Rows";
import css from "./ArtifactFoundTree.module.css";

// ArtifactFoundTree is the artifacts found in the repos as a tree: a repo,
// the folders its artifacts are grouped in, and theirs (an
// ArtifactFoundFolder each). It isn't the library's Tree: what's in a
// folder is a list of an artifact's own rows, not a line.
export function ArtifactFoundTree({ children }: { children: ReactNode }) {
  return <div className={css.tree}>{children}</div>;
}

type ArtifactFoundFolderProps = {
  // What it's called, how many artifacts are in it and in its folders,
  // and where it is, said on hover.
  name: string;
  count: number;
  title: string;
  // Whether it's unfolded, and what folding or unfolding it does.
  open: boolean;
  onToggle: (open: boolean) => void;
  // The artifacts straight in it: an ArtifactRow each.
  rows?: ReactNode;
  // The folders in it.
  children?: ReactNode;
};

// ArtifactFoundFolder is a repo, or a folder of its artifacts: its name
// and how many are in it, and, unfolded, the artifacts straight in it and
// its folders, in from it down a line.
export function ArtifactFoundFolder({ name, count, title, open, onToggle, rows, children }: ArtifactFoundFolderProps) {
  return (
    <details className={css.node} open={open} onToggle={(e) => onToggle(e.currentTarget.open)}>
      <summary title={title}>
        <span className={css.name}>{name}</span>
        <span className={css.count}>{count}</span>
      </summary>
      <div className={css.body}>
        {rows && <Rows className={css.rows}>{rows}</Rows>}
        {children}
      </div>
    </details>
  );
}
