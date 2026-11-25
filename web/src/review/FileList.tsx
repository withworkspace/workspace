import { useMemo } from "react";
import type { DiffFile } from "../api";
import { Icon, type IconName } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { groupByOrder, groupFiles, type Order } from "./files";
import { ReviewFiles } from "./ReviewFiles";
import type { FileState } from "./model";
import { Button } from "../ui/Button";
import { ToggleButton } from "../ui/ToggleButton";

// ViewEntry is one of a review's views in the list down the side: what's
// read beside the changes (the overview, the guide, what's been said, the
// passes), each shown alone in the pane.
export interface ViewEntry {
  key: string;
  label: string;
  icon: IconName;
  n: number; // its key: 1 for the first
  mark?: React.ReactNode; // how it stands (each pass), beside its name
  count?: number; // how much is in it (what's been said)
  news?: string; // what's new in it since you looked
}

// ViewList is the review's views, above the changes.
export function ViewList({ views, here, onPick }: { views: ViewEntry[]; here: string; onPick: (key: string) => void }) {
  if (!views.length) return null;
  return (
    <ul className="files-views">
      {views.map((v) => (
        <li key={v.key} className={here === v.key ? "here" : ""} {...keyProps(`${v.n}`, `${v.label}${v.news ? `: ${v.news}` : ""}`, () => onPick(v.key))} aria-current={here === v.key ? "page" : undefined}>
          <span className="fstatus s-view">
            <Icon name={v.icon} size={11} />
          </span>
          <span className="fname plain">{v.label}</span>
          {v.mark && <span className="view-mark">{v.mark}</span>}
          {v.count ? <span className="view-count">{v.count}</span> : null}
          {v.news && <span className="fnews" aria-label={v.news} />}
        </li>
      ))}
    </ul>
  );
}

// FileList is the Changes view's entry and, under it, the diff's files in
// groups: a folder's files under the folder, each row the file's name. A
// folder is cut short from its left, so what's nearest the file is what's
// kept.
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
      <ul className="files-views files-changes">
        <li className={current ? "here" : ""} {...keyProps(`${n}`, "Changes", onChanges)} aria-current={current ? "page" : undefined}>
          <span className="fstatus s-view">
            <Icon name="code" size={11} />
          </span>
          <span className="fname plain">Changes</span>
          <span className="view-count">
            {viewedCount}/{files.length}
          </span>
          <span className="files-tools" onClick={(e) => e.stopPropagation()}>
            {onOrder && (
              <ToggleButton
                variant="ghost" size="small"
                className="files-order"
                pressed={!!order}
                onChange={onOrder}
                title={order ? "The files are in the guide's order: switch to their folders'" : "Read the files in the guide's order"}
              >
                <Icon name="sparkle" size={12} /> Guide
              </ToggleButton>
            )}
            <Button variant="ghost" size="small" square onClick={onFind} title="Jump to a file" keys="t" aria-label="Jump to a file">
              <Icon name="search" size={13} />
            </Button>
          </span>
        </li>
      </ul>
      <div className="progress" title={`${viewedCount} of ${files.length} ${files.length === 1 ? "file" : "files"} viewed`}>
        <div style={{ width: files.length ? `${(100 * viewedCount) / files.length}%` : 0 }} />
      </div>
      {common && (
        <div className="files-common cut-left" title={`Every file to read is under ${common}`}>
          <bdi>{common}</bdi>
        </div>
      )}
      <ReviewFiles files={files} groups={groups} states={states} here={current ? here : undefined} counts={counts} found={found} notes={guideNotes} onPick={onPick} />
    </>
  );
}
