import { useCallback, useMemo } from "react";
import type { DiffFile } from "../api";
import { ChangeMark } from "../ui/ChangeMark";
import { Count } from "../ui/Count";
import { FileIcon } from "../ui/FileIcon";
import { Icon } from "../ui/Icon";
import { Tree } from "../ui/Tree";
import { TreeMark } from "../ui/TreeMark";
import type { TreeLook } from "../ui/TreeRow";
import type { TreeNode } from "../ui/treeNodes";
import { dirOf, type FileGroup } from "./files";
import type { FileState } from "./model";
import css from "./ReviewFiles.module.css";
import { type FilesRow, reviewFilesTree } from "./reviewFilesTree";

// ReviewFiles is a review's files, down the side (ui/Tree draws them): in
// groups, in the order they're read in, each group folding over its
// files. A file says how it's changed, what's been said on it and what
// passes found, how much of it changed, and that you've viewed it; one
// viewed is quiet. A group to skim is quiet too, and says so.
//
// The keys here are the page's, on the diff: a file picked is gone to
// there (onPick), and the keys go with it.
export function ReviewFiles({
  files,
  groups,
  states,
  here,
  counts,
  found,
  notes,
  onPick,
}: {
  files: DiffFile[];
  groups: FileGroup[];
  states: FileState[];
  here?: number; // the file the cursor's in, where the diff's showing
  counts: Record<string, number>; // comments, by path
  found?: Record<string, number>; // what passes found that's open, by path
  notes?: Record<string, string>; // a guide's line on each file, by path
  onPick: (key: string) => void;
}) {
  const nodes = useMemo(() => reviewFilesTree(files, groups), [files, groups]);
  const look = useCallback(
    (n: TreeNode<FilesRow>, at: { open: boolean }): TreeLook => {
      const v = n.value!;
      if ("group" in v) {
        const g = v.group;
        return {
          title: g.title,
          tone: g.skim ? "quiet" : undefined,
          end: (
            <>
              {g.named && g.skim && <span className={css.said}>to skim</span>}
              {!at.open && <Count title={`${g.files.length} ${g.files.length === 1 ? "file" : "files"}`}>{g.files.length}</Count>}
            </>
          ),
        };
      }
      const f = files[v.file];
      const s = states[v.file];
      const said = counts[f.path] ?? 0;
      const open = found?.[f.path] ?? 0;
      const g = groups.find((x) => x.files.includes(v.file));
      return {
        mark: <ChangeMark status={f.status} />,
        icon: <FileIcon path={f.path} />,
        // (One deleted isn't struck, as the explorer's is: it's still to read.)
        tone: s?.viewed || s?.unchanged ? "quiet" : undefined,
        title: `${f.path}${s?.unchanged ? ": unchanged since you reviewed it" : notes?.[f.path] ? `: ${notes[f.path]}` : ""}`,
        end: (
          <>
            {(g?.skim || g?.named) && dirOf(f.path) && <span className={css.dir}>{dirOf(f.path)}</span>}
            {open > 0 && (
              <span className={css.found} title={`${open} ${open === 1 ? "finding" : "findings"} of a pass, still open`}>
                <Icon name="sparkle" size={11} />
                {open}
              </span>
            )}
            {said > 0 && (
              <Count tone="attention" title={`${said} ${said === 1 ? "comment" : "comments"}`}>
                {said}
              </Count>
            )}
            <span className={css.counts}>
              {f.additions > 0 && <span className={css.plus}>+{f.additions}</span>}
              {f.deletions > 0 && <span className={css.minus}>−{f.deletions}</span>}
            </span>
            <span className={css.viewed}>
              {s?.viewed && (
                <TreeMark tone="done" title="Viewed">
                  <Icon name="check" size={11} />
                </TreeMark>
              )}
            </span>
          </>
        ),
      };
    },
    [files, groups, states, counts, found, notes],
  );
  return (
    <Tree
      // The groups it starts open on are these: another order's are others.
      key={groups.map((g) => g.key).join("\n")}
      aria-label="Changed files"
      className={css.files}
      nodes={nodes}
      look={look}
      startOpen="all"
      current={here === undefined ? undefined : `${here}`}
      onPick={(n) => {
        onPick(n.id);
        // The page's keys are on the diff, where it's gone: not left here.
        (document.activeElement as HTMLElement | null)?.blur();
      }}
      typeahead={false}
      guides={false}
      marks
      mono
    />
  );
}
