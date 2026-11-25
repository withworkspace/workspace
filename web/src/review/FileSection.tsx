import { memo, useCallback, useMemo } from "react";
import type { DiffFile, NoteDraft } from "../api";
import type { DiffAt } from "../ui/DiffLine";
import type { DiffCell, DiffPick } from "../ui/diffLayout";
import { CommentCard } from "./CommentCard";
import { FindingCard } from "./FindingCard";
import type { FindingAct } from "./findings";
import { skimWords } from "./files";
import { highlight } from "./highlightDiff";
import { type Cell, cellHTML, type Editing, type FileState, type Row } from "./model";
import { ReviewFile } from "./ReviewFile";
import { ReviewLines } from "./ReviewLines";
import { ReviewNoteEditor } from "./ReviewNoteEditor";
import { ReviewSelectBar } from "./ReviewSelectBar";

interface FileSectionProps {
  index: number;
  file: DiffFile;
  rows: Row[];
  state: FileState;
  split: boolean;
  cursor: string; // the cursor's row, if it's in this file
  selected?: Set<string>;
  selectSide?: "old" | "new"; // side by side, the column the lines were picked in
  selectionEnd?: string; // the selection's last row, where its bar goes
  selectionLabel?: string;
  editing?: Editing;
  highlighting: boolean;
  onRow: (key: string) => void;
  onActivate: (r: Row) => void;
  onSave: (n: NoteDraft) => void;
  onCancel: () => void;
  onComment?: () => void; // absent where comments can't be made
  onAsk?: () => void; // ask an agent about the selected lines
  onFold: (file: number) => void; // open or fold a file unchanged since you reviewed it
  onRowClick: (key: string, shift: boolean) => void;
  onPick: (pick: DiffPick) => void; // lines picked by their numbers
  replying?: string; // the comment you're answering
  onAnswer: (id: string, what: "reply" | "resolve" | "reopen", body?: string) => void;
  onReplyCancel: () => void;
  dismissing?: string; // the finding being dismissed, if it's in this file
  onAct: FindingAct;
  onDismissing: (key?: string) => void;
  followups?: Record<string, { kind: string; text: string; blocks?: boolean }>;
  note?: string; // what the change does to it, in a line (a guide's)
  navAt?: DiffAt;
}

// FileSection is one of a review's files, as the page has it: coloured,
// its rows drawn by ReviewLines (a comment, a finding, the editor among
// them) and the lines picked with a bar under them. It draws again only
// when its own rows, cursor or selection change.
export const FileSection = memo(function FileSection(props: FileSectionProps) {
  const { index, file, rows, state, split, cursor, selected, selectSide, selectionEnd, selectionLabel, editing, highlighting } = props;
  const { onRow, onActivate, onSave, onCancel, onComment, onAsk, onFold, onRowClick, onPick, replying, onAnswer, onReplyCancel, dismissing, onAct, onDismissing, followups, note, navAt } = props;
  // highlighting is a dependency: once the highlighter loads, colour.
  const hl = useMemo(() => highlight(file, state.blob), [file, state.blob, highlighting]);
  const html = useCallback((c: DiffCell, side: "old" | "new") => cellHTML(hl, c as Cell, side), [hl]);
  // Its rows under its head, and the lines picked among them: from the
  // first to the last, as the part that draws them has it.
  const lines = useMemo(() => rows.slice(1), [rows]);
  const picked = useMemo((): DiffPick | undefined => {
    if (!selected?.size) return undefined;
    const from = lines.find((r) => selected.has(r.key));
    const to = lines.findLast((r) => selected.has(r.key));
    return from && to ? { from: from.key, to: to.key, side: selectSide } : undefined;
  }, [lines, selected, selectSide]);
  // The lines the editor is on, for a suggestion to start from.
  const editorLines = () => {
    if (!editing) return [];
    if (editing.note) return editing.note.excerpt;
    const t = editing.target;
    const out: string[] = [];
    for (const r of rows) {
      if (r.type !== "line") continue;
      const c = t.side === "new" ? r.right : r.left?.kind === "d" ? r.left : undefined;
      if (c && c.num >= t.line && c.num <= t.endLine) out.push(c.text);
    }
    return out;
  };
  return (
    <ReviewFile
      index={index}
      status={file.status}
      path={file.path}
      oldPath={file.oldPath}
      additions={file.additions}
      deletions={file.deletions}
      viewed={!!state.viewed}
      folded={!!state.folded}
      cursor={cursor === `${index}`}
      unchanged={state.unchanged}
      skim={file.skim ? skimWords[file.skim] : undefined}
      guide={note}
      onHead={() => onRow(`${index}`)}
      onFold={() => onFold(index)}
      onViewed={() => onActivate({ type: "file", key: `${index}`, file: index })}
    >
      {!state.viewed && !state.folded && (
        <ReviewLines
          label={`Changes to ${file.path}`}
          rows={lines}
          split={split}
          html={html}
          at={navAt}
          cursor={cursor || undefined}
          picked={picked}
          onPick={onPick}
          // A number's press is a pick, told already; the editor's rows
          // aren't somewhere the keys go.
          onRow={(key, how) => !how.number && !key.endsWith(":editor") && onRowClick(key, how.shift)}
          onGap={onActivate}
          onNote={onActivate}
          draw={(r) =>
            r.type === "editor" && editing ? (
              <ReviewNoteEditor target={editing.target} note={editing.note} lines={editorLines()} onSave={onSave} onCancel={onCancel} />
            ) : r.type === "comment" ? (
              <CommentCard row={r} cursor={cursor === r.key} replying={replying === r.placed.note.id} onRow={onRow} onAnswer={onAnswer} onReplyCancel={onReplyCancel} followup={r.placed.draft ? undefined : followups?.[r.placed.note.id]} />
            ) : r.type === "finding" ? (
              <FindingCard row={r} cursor={cursor === r.key} dismissing={dismissing === r.key} canAsk={!!onAsk} onRow={onRow} onAct={onAct} onDismissing={onDismissing} />
            ) : null
          }
          under={selectionEnd && selectionLabel ? { row: selectionEnd, what: <ReviewSelectBar label={selectionLabel} onComment={onComment} onAsk={onAsk} /> } : undefined}
        />
      )}
    </ReviewFile>
  );
});
