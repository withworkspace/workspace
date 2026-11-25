import type { DiffFile } from "../api";
import type { TreeNode } from "../ui/treeNodes";
import { baseOf, type FileGroup } from "./files";

// A review's files as a tree's rows (ui/Tree draws them; ReviewFiles says
// how they look): its groups, in the order they're read in, each a row
// that folds over its files. It's not the repo's folders: a group is a
// run of files read together (a folder's, a guide's, those to skim).

// FilesRow is what a row is of: a group, or a file by its place in the
// diff's files.
export type FilesRow = { group: FileGroup } | { file: number };

// A group's name as its row says it: a folder's without its last slash
// (the tree draws a folder's slashes quiet), "./" for the top's.
const groupName = (g: FileGroup) => (g.named || g.skim ? g.label : g.label.replace(/\/$/, "") || "./");

// reviewFilesTree is the rows: a file's id is its place in the diff's
// files, which is its row's key in the diff too. One group that isn't to
// skim has no row of its own: its files are all there is.
export function reviewFilesTree(files: DiffFile[], groups: FileGroup[]): TreeNode<FilesRow>[] {
  const leaf = (i: number): TreeNode<FilesRow> => ({ id: `${i}`, name: baseOf(files[i].path), value: { file: i } });
  if (groups.length === 1 && !groups[0].skim) return groups[0].files.map(leaf);
  return groups.map((g) => ({ id: `group:${g.key}`, name: groupName(g), value: { group: g }, children: g.files.map(leaf) }));
}
