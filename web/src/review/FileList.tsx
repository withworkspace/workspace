import { useMemo } from "react";
import type { DiffFile } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { ToggleButton } from "../ui/ToggleButton";
import { groupByOrder, groupFiles, type Order } from "./files";
import type { FileState } from "./model";
import { ReviewCommon } from "./ReviewCommon";
import { ReviewFiles } from "./ReviewFiles";
import { ReviewProgress } from "./ReviewProgress";
import { ReviewViewRow } from "./ReviewViewRow";
import { ReviewViews } from "./ReviewViews";
import css from "./FileList.module.css";

// FileList is the Changes view's entry and, under it, the diff's files in
// groups: a folder's files under the folder, or the guide's groups in its
// order, each row the file's name.
export function FileList({
  files,
  states,
  here,
  current,
  n,
  counts,
  found,
  order,
  guideNotes,
  onOrder,
  onChanges,
  onPick,
  onFind,
}: {
  files: DiffFile[];
  states: FileState[];
  here?: number; // the file the cursor is in
  current: boolean; // the Changes view is the one showing
  n: number; // the Changes view's key
  counts: Record<string, number>; // comments, by path
  found?: Record<string, number>; // what passes found that's open, by path
  // order is a guide's order, when the files are in it; onOrder switches
  // between it and their folders', where there's a guide to follow.
  order?: Order;
  guideNotes?: Record<string, string>; // the guide's line on each file, by path
  onOrder?: (guided: boolean) => void;
  onChanges: () => void; // show the Changes view
  onPick: (key: string) => void; // the row to put the cursor on
  onFind: () => void;
}) {
  const { common, groups } = useMemo(() => (order ? groupByOrder(files, order) : groupFiles(files)), [files, order]);
  const viewedCount = states.filter((s) => s.viewed).length;
  return (
    <>
      <ReviewViews>
        <ReviewViewRow
          icon="code"
          label="Changes"
          n={n}
          here={current}
          count={
            <>
              {viewedCount}/{files.length}
            </>
          }
          onPick={onChanges}
          tools={
            <>
              {onOrder && (
                <ToggleButton
                  variant="ghost"
                  size="small"
                  className={css.order}
                  pressed={!!order}
                  onChange={onOrder}
                  title={order ? "Switch to folder order" : "Guide order"}
                >
                  <Icon name="sparkle" size={12} /> Guide
                </ToggleButton>
              )}
              <Button variant="ghost" size="small" square onClick={onFind} title="Jump to a file" keys="t" aria-label="Jump to a file">
                <Icon name="search" size={13} />
              </Button>
            </>
          }
        />
      </ReviewViews>
      <ReviewProgress viewed={viewedCount} of={files.length} />
      {common && <ReviewCommon path={common} />}
      <ReviewFiles files={files} groups={groups} states={states} here={current ? here : undefined} counts={counts} found={found} notes={guideNotes} onPick={onPick} />
    </>
  );
}
