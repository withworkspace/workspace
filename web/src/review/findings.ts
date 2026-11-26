import type { DismissWhy, PassFinding } from "../api";

// What a pass found, in the diff: a card under the lines it's about, like
// a comment's, but nobody's said it yet. Taking it puts it in your draft
// as a comment; dismissing it says why.

// Why a review's finding is dismissed: what a page's is, as a reviewer
// would say it.
export const reviewDismissWhys: [DismissWhy, string, string][] = [
  ["wrong", "Wrong", "Not true"],
  ["already", "Said already", "Already raised or known"],
  ["minor", "Not worth it", "True, but not worth raising"],
];

// What a follow-up says of a comment.
export const followWords: Record<string, string> = { addressed: "Addressed", partly: "Partly addressed", "not-addressed": "Not addressed" };

// What's done with a finding: taken into your draft, dismissed (and why),
// or asked about.
export type FindingAct = (found: { pass: string; finding: PassFinding }, what: "taken" | "dismissed" | "open" | "ask", why?: DismissWhy) => void;

// linesWord says which lines something's on: one, or from one to another.
export const linesWord = (t: { line: number; endLine: number }) => (t.endLine > t.line ? `lines ${t.line}–${t.endLine}` : `line ${t.line}`);
