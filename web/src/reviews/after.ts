import type { PR, ReviewRow } from "../api";

// What follows posting a review: back to the list, for the next one, unless
// the stack it's in still has a layer that needs you.

// layersLeft is the layers of a stack that still need you, once the reviews
// of posted (their numbers) are on GitHub. A pull request on its own has no
// layers, so none are left.
export function layersLeft(layers: PR[], posted: number[]): PR[] {
  return layers.filter((l) => (l.state === "needs-review" || l.state === "back-with-you") && !posted.includes(l.number));
}

// rowAfter is the row to select in the list once repo#number has been
// reviewed: the one below its row, as the list was, or the one above for
// the last. Its own row moves to another section when the list is next
// read, and the selection shouldn't go with it.
export function rowAfter(rows: ReviewRow[], repo: string, number: number): string | undefined {
  const holds = (p: PR) => p.repo === repo && p.number === number;
  const i = rows.findIndex((r) => (r.pr ? holds(r.pr) : !!r.stack?.layers.some(holds)));
  if (i < 0) return undefined;
  return (rows[i + 1] ?? rows[i - 1])?.key;
}
