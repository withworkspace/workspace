import type { TreeFile } from "../../api";
import { ChangeMark } from "../../ui/ChangeMark";
import { Count } from "../../ui/Count";
import { FileIcon } from "../../ui/FileIcon";
import { TreeDot } from "../../ui/TreeDot";
import type { TreeLook } from "../../ui/TreeRow";
import type { TreeNode } from "../../ui/treeNodes";
import { statusWords } from "./words";

// How the rows of the explorer's tree look (ui/Tree draws them; ./tree
// makes it of a worktree's files).

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
  return {
    mark: f.status && <ChangeMark status={f.status} title={`${statusWords[f.status] ?? f.status} since the branch point${f.uncommitted ? ", not committed" : ""}`} />,
    icon: <FileIcon path={n.id} />,
    tone: f.status === "deleted" ? "gone" : undefined,
    end: f.uncommitted && <TreeDot tone="attention" title="Not committed" />,
  };
}
