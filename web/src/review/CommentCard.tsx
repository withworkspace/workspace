import type { Row } from "./model";
import { ReviewComment } from "./ReviewComment";

// CommentCard draws a comment's row as a ReviewComment: where it's
// placed, and what's said to it, made the page's.
export function CommentCard({
  row,
  cursor,
  replying,
  onRow,
  onAnswer,
  onReplyCancel,
  followup,
}: {
  row: Extract<Row, { type: "comment" }>;
  followup?: { kind: string; text: string; blocks?: boolean }; // what the follow-up said of it
  cursor: boolean;
  replying: boolean;
  onRow: (key: string) => void;
  onAnswer: (id: string, what: "reply" | "resolve" | "reopen", body?: string) => void;
  onReplyCancel: () => void;
}) {
  const { note, draft } = row.placed;
  return (
    <ReviewComment
      id={`row-${row.key}`}
      note={note}
      draft={draft}
      apart={row.unplaced}
      elsewhere={!row.placed.at}
      cursor={cursor}
      replying={replying}
      followup={followup}
      onClick={() => onRow(row.key)}
      onToggle={() => {
        onRow(row.key);
        onAnswer(note.id, note.status === "resolved" ? "reopen" : "resolve");
      }}
      onReply={(body) => onAnswer(note.id, "reply", body)}
      onReplyCancel={onReplyCancel}
    />
  );
}
