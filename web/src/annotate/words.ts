import type { ThreadKind } from "../api";

// What a document's threads are called.

export const kindWords: Record<ThreadKind, string> = { ask: "Ask", change: "Change", note: "Note" };
export const kindIcon: Record<ThreadKind, "question" | "pencil" | "message"> = { ask: "question", change: "pencil", note: "message" };
export const kindHints: Record<ThreadKind, string> = {
  ask: "What do you want to know?",
  change: "What should change?",
  note: "What do you want to say?",
};

export const linesWord = (n: { line: number; endLine: number }) => (n.endLine > n.line ? `lines ${n.line}–${n.endLine}` : `line ${n.line}`);
