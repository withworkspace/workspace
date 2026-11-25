import type { ReactNode } from "react";
import type { TreeFile } from "../../api";
import { Count } from "../../ui/Count";
import { FileIcon } from "../../ui/FileIcon";
import { Icon } from "../../ui/Icon";
import { TreeDot } from "../../ui/TreeDot";
import { TreeMark, type TreeTone } from "../../ui/TreeMark";
import type { TreeLook } from "../../ui/TreeRow";
import type { TreeNode } from "../../ui/treeNodes";
import { type FileStatus, statusWords } from "./words";

// How the rows of the explorer's tree look (ui/Tree draws them; ./tree
// makes it of a worktree's files).

const marks: Record<FileStatus, { tone: TreeTone; mark: ReactNode }> = {
  added: { tone: "done", mark: "A" },
  modified: { tone: "attention", mark: "M" },
  deleted: { tone: "danger", mark: "D" },
  renamed: { tone: "accent", mark: <Icon name="arrow-right" size={10} /> },
  copied: { tone: "accent", mark: <Icon name="copy" size={10} /> },
  type: { tone: "attention", mark: "T" },
};

// filesTreeLook is how a row of it looks. A file has its mark (how it's
// changed since the branch point), its icon, and a dot while that isn't
// committed; one deleted is struck. A folder says how many changed files
// are in it while it's folded (counts: none are given while only what's
// changed is shown, where every file is one).
export function filesTreeLook(n: TreeNode<TreeFile>, at: { open: boolean }, counts?: ReadonlyMap<string, number>): TreeLook {
  if (n.children) {
    const changed = counts?.get(n.id) ?? 0;
    return { end: !at.open && changed > 0 && <Count title={`${changed} changed in it`}>{changed}</Count> };
  }
  const f = n.value!;
  const m = f.status && marks[f.status];
  return {
    mark: m && (
      <TreeMark tone={m.tone} title={`${statusWords[f.status!] ?? f.status} since the branch point${f.uncommitted ? ", not committed" : ""}`}>
        {m.mark}
      </TreeMark>
    ),
    icon: <FileIcon path={n.id} />,
    tone: f.status === "deleted" ? "gone" : undefined,
    end: f.uncommitted && <TreeDot tone="attention" title="Not committed" />,
  };
}
