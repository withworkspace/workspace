import { useMemo } from "react";
import type { DiffFile } from "../api";
import { Icon, type IconName } from "../ui/Icon";
import { baseOf, dirOf, groupByOrder, groupFiles, type Order } from "./files";
import type { FileState } from "./model";

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
        <li key={v.key} className={here === v.key ? "here" : ""} onClick={() => onPick(v.key)} title={`${v.label}${v.news ? `: ${v.news}` : ""} (${v.n})`} aria-current={here === v.key ? "page" : undefined}>
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
        <li className={current ? "here" : ""} onClick={onChanges} title={`Changes (${n})`} aria-current={current ? "page" : undefined}>
          <span className="fstatus s-view">
            <Icon name="code" size={11} />
          </span>
          <span className="fname plain">Changes</span>
          <span className="view-count">
            {viewedCount}/{files.length}
          </span>
          <span className="files-tools" onClick={(e) => e.stopPropagation()}>
            {onOrder && (
              <button
                className={`ghost small files-order${order ? " on" : ""}`}
                aria-pressed={!!order}
                onClick={() => onOrder(!order)}
                title={order ? "The files are in the guide's order: switch to their folders'" : "Read the files in the guide's order"}
              >
                <Icon name="sparkle" size={12} /> Guide
              </button>
            )}
            <button className="ghost icon-button small" onClick={onFind} title="Jump to a file (t)" aria-label="Jump to a file">
              <Icon name="search" size={13} />
            </button>
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
      {groups.map((g) => (
        <section key={g.key} className="file-group">
          {(groups.length > 1 || g.skim) && (
            <div className={`file-group-head${g.skim || g.named ? " skim" : " cut-left"}`} title={g.title}>
              <bdi>{g.label || "./"}</bdi>
              {g.named && g.skim && <span className="dim"> · to skim</span>}
            </div>
          )}
          <ul>
            {g.files.map((i) => {
              const f = files[i];
              const s = states[i];
              const n = counts[f.path] ?? 0;
              const fn = found?.[f.path] ?? 0;
              return (
                <li
                  key={f.path}
                  className={`${current && here === i ? "here" : ""} ${s.viewed || s.unchanged ? "viewed" : ""}`}
                  onClick={() => onPick(`${i}`)}
                  title={`${f.path}${s.unchanged ? ": unchanged since you reviewed it" : guideNotes?.[f.path] ? `: ${guideNotes[f.path]}` : ""}`}
                >
                  <span className={`fstatus s-${f.status}`} title={f.status}>
                    {f.status === "added" ? <Icon name="plus" size={11} /> : f.status === "deleted" ? <Icon name="minus" size={11} /> : f.status === "renamed" ? <Icon name="arrow-right" size={11} /> : <span className="fdot" />}
                  </span>
                  <span className="fname">{baseOf(f.path)}</span>
                  {(g.skim || g.named) && (
                    <span className="fdir cut-left">
                      <bdi>{dirOf(f.path)}</bdi>
                    </span>
                  )}
                  {fn > 0 && (
                    <span className="note-count found-count" title={`${fn} ${fn === 1 ? "finding" : "findings"} of a pass, still open`}>
                      <Icon name="sparkle" size={11} />
                      {fn}
                    </span>
                  )}
                  {n > 0 && (
                    <span className="note-count" title={`${n} ${n === 1 ? "comment" : "comments"}`}>
                      <Icon name="message" size={11} />
                      {n}
                    </span>
                  )}
                  <span className="counts">
                    {f.additions > 0 && <span className="plus">+{f.additions}</span>}
                    {f.deletions > 0 && <span className="minus">−{f.deletions}</span>}
                  </span>
                  <span className="check">{s.viewed ? <Icon name="check" size={12} /> : null}</span>
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </>
  );
}
