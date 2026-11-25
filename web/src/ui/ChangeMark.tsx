import type { ReactNode } from "react";
import { Icon } from "./Icon";
import { TreeMark, type TreeTone } from "./TreeMark";

export type ChangeStatus = "added" | "modified" | "deleted" | "renamed" | "copied" | "type";

const marks: Record<ChangeStatus, { tone: TreeTone; mark: ReactNode; words: string }> = {
  added: { tone: "done", mark: "A", words: "Added" },
  modified: { tone: "attention", mark: "M", words: "Modified" },
  deleted: { tone: "danger", mark: "D", words: "Deleted" },
  renamed: { tone: "accent", mark: <Icon name="arrow-right" size={10} />, words: "Renamed" },
  copied: { tone: "accent", mark: <Icon name="copy" size={10} />, words: "Copied" },
  type: { tone: "attention", mark: "T", words: "Changed type" },
};

// ChangeMark is how a file's changed, as a tree's row marks it (a
// TreeMark): A, M or D in its colour, an arrow for one renamed. The same
// wherever files are listed in a tree: the explorer's, a review's. title
// says it in the page's words (since when), where "Added" isn't enough.
export function ChangeMark({ status, title }: { status: ChangeStatus; title?: string }) {
  const m = marks[status];
  return (
    <TreeMark tone={m.tone} title={title ?? m.words}>
      {m.mark}
    </TreeMark>
  );
}
