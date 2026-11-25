import type { ThreadKind } from "../api";

// What a document's threads are called, and the keys that comment on it.

export const annotateKeys: [string, string][] = [
  ["select, then c", "Comment on the text selected"],
  ["j / k", "Next / previous paragraph"],
  ["⇧↓ / ⇧↑", "Take in the next / previous paragraph"],
  ["c", "Comment on the paragraphs (or the text selected)"],
  ["n / p", "Next / previous open thread"],
  ["⏎", "Reply"],
  ["a / d", "Accept / decline the thread's suggested edit"],
  ["v", "See the suggested edits in place, as tracked changes"],
  ["x", "Resolve / reopen the thread"],
  ["i", "Show / hide what's changed since you last commented"],
  ["⇧A", "Approve the document"],
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
