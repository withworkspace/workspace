import type { CrewDetail } from "../../api";
import type { Work, WorkCapsule } from "../workRows";

// What the Work tab says in words, and where it points.

// bundleLink is a bundle, in the mission's Verify tab.
export const bundleLink = (slug: string, id: string) => `/missions/${slug}?${new URLSearchParams({ tab: "verify", b: id })}`;

// count is how many of something: 1 commit, 3 files.
export const count = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? "" : "s"}`;

// workWords is what Summary says of the Work tab in a line: what's up
// next and done, and how many outputs.
export function workWords(w: Work, label: (repo: string) => string, slug: string): string {
  const words = [
    w.next.length > 0 && `up next: ${doneWords(w.next, label, slug)}`,
    w.done.length > 0 && `done: ${doneWords(w.done, label, slug)}`,
    w.outputs.length > 0 && `${w.outputs.length} ${w.outputs.length === 1 ? "output" : "outputs"}`,
  ].filter(Boolean);
  return words.join(" · ").replace(/^./, (c) => c.toUpperCase());
}

// doneWords is capsules in a line, as their rows name them (the repo, and
// the capsule's own name when it isn't the mission's), each once, with how
// many when it's more than one.
export function doneWords(done: WorkCapsule[], label: (repo: string) => string, slug: string): string {
  const n = new Map<string, number>();
  for (const x of done) {
    const name = label(x.c.repo) + (x.c.name === slug ? "" : ` ${x.c.name}`);
    n.set(name, (n.get(name) ?? 0) + 1);
  }
  return [...n].map(([name, k]) => (k > 1 ? `${name} ×${k}` : name)).join(", ");
}

// howEnded is how a crew ended, in a word or two: the whole of it is said
// on hover.
export function howEnded(k: CrewDetail): string {
  switch (k.how) {
    case "handed-back":
      return "Handed back";
    case "interrupted":
      return "Stopped by a restart";
    case "stopped":
      return "Stopped";
  }
  return k.status === "done" ? "Done" : "Ended";
}
