import type { ThreadKind } from "../api";

// What a document's threads are called, and the keys that comment on it.

export const annotateKeys: [string, string][] = [
  ["select, then c", "Comment on selection"],
  ["j / k", "Next / previous paragraph"],
  ["⇧↓ / ⇧↑", "Extend to next / previous paragraph"],
  ["c", "Comment on paragraphs or selection"],
  ["n / p", "Next / previous open thread"],
  ["⏎", "Reply"],
  ["a / d", "Accept / decline suggested edit"],
  ["v", "Show suggested edits as tracked changes"],
  ["x", "Resolve / reopen thread"],
  ["i", "Show / hide changes since last comment"],
  ["⇧A", "Approve document"],
  ["tab", "While writing: ask, change or note"],
];

export const kindWords: Record<ThreadKind, string> = { ask: "Ask", change: "Change", note: "Note" };
export const kindIcon: Record<ThreadKind, "question" | "pencil" | "message"> = { ask: "question", change: "pencil", note: "message" };
export const kindHints: Record<ThreadKind, string> = {
  ask: "What do you want to know?",
  change: "What should change?",
  note: "What do you want to say?",
};

export const linesWord = (n: { line: number; endLine: number }) => (n.endLine > n.line ? `lines ${n.line}–${n.endLine}` : `line ${n.line}`);
