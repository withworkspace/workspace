import type { Verdict } from "../api";

// What a capsule's review and a document's share: how verdicts and comment
// states read, and suggestions as written in a comment.

export const verdictWords: Record<Verdict, string> = { approve: "approved", changes: "changes requested", comment: "comments" };

export const statusWords: Record<string, string> = { open: "Open", addressed: "Addressed", wontfix: "Won't fix", resolved: "Resolved" };

// A suggestion is written as a ```suggestion block in the comment, as on
// GitHub; it's kept apart from the words.
export const suggestionBlock = /```suggestion\n([\s\S]*?)\n?```\n?/;

export function splitSuggestion(text: string): { body: string; suggestion: string | null } {
  const m = text.match(suggestionBlock);
  if (!m) return { body: text.trim(), suggestion: null };
  return { body: text.replace(suggestionBlock, "").trim(), suggestion: m[1] };
}

export function joinSuggestion(body: string, suggestion?: string | null) {
  return suggestion == null ? body : `${body}${body ? "\n\n" : ""}\`\`\`suggestion\n${suggestion}\n\`\`\``;
}
