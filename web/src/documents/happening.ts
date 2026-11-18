import type { Space, SpaceActivity, SpaceCommit, SpaceDoc, SpaceDocChange, SpaceFlight } from "../api";

// What the documents' pages say of what's happening in a space, worked out
// apart from how it's drawn.

// saysNoMore is whether a commit's subject says nothing its document's
// line doesn't: its title or its file's name, and what was done to it
// ("Add doc: Verify v2", "Amend doc verify-v2"). Such a subject isn't
// shown beside it. What's left once the name's taken out has to be only
// words for doing something to a document, its kind, or short words (a,
// of, adr) and numbers.
export function saysNoMore(subject: string, d: SpaceDocChange): boolean {
  const words = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
  const name = words(
    d.path
      .slice(d.path.lastIndexOf("/") + 1)
      .replace(/\.md$/i, "")
      .replace(/^\d{4}-\d{2}-\d{2}-/, "")
      .replace(/^\d+-/, ""),
  );
  let left = ` ${words(subject)} `;
  for (const said of [d.title ? words(d.title) : "", name]) if (said && left.includes(` ${said} `)) left = left.replace(` ${said} `, " ");
  if (left.length === ` ${words(subject)} `.length) return false; // it doesn't name it
  return left
    .trim()
    .split(" ")
    .filter(Boolean)
    .every((w) => /^\d+$/.test(w) || w.length <= 3 || doing.has(w));
}

const doing = new Set(
  "add added adds amend amended amends edit edited edits update updated updates change changed changes fix fixed fixes new remove removed removes rename renamed move moved file filed tidy tidied draft drafted rewrite rewrote reword reworded doc docs plan plans page pages note notes guide guides".split(" "),
);

// A commit or a capsule, with the space it's in.
export type InSpace<T> = T & { repo: string };

// acrossSpaces is what changed lately in every space, newest first, and
// what's in flight in each, from the activity read of each.
export function acrossSpaces(spaces: Space[], activity: Map<string, SpaceActivity>): { lately: InSpace<SpaceCommit>[]; inFlight: InSpace<SpaceFlight>[] } {
  const lately: InSpace<SpaceCommit>[] = [];
  const inFlight: InSpace<SpaceFlight>[] = [];
  for (const s of spaces) {
    const a = activity.get(s.repo);
    for (const c of a?.lately ?? []) lately.push({ ...c, repo: s.repo });
    for (const f of a?.inFlight ?? []) inFlight.push({ ...f, repo: s.repo });
  }
  lately.sort((a, b) => b.at.localeCompare(a.at));
  return { lately, inFlight };
}

// changedSince is how many of the commits were made after a time.
export function changedSince(lately: SpaceCommit[], since: number): number {
  return lately.filter((c) => new Date(c.at).getTime() > since).length;
}

// lastChanged is the document changed last of those given, if one says
// when.
export function lastChanged(docs: SpaceDoc[]): SpaceDoc | undefined {
  let last: SpaceDoc | undefined;
  for (const d of docs) if (d.changed && (!last?.changed || d.changed > last.changed)) last = d;
  return last;
}

// withoutTitle is a document's text without the heading that's its
// title, which the preview says above it.
export function withoutTitle(text: string): string {
  return text.replace(/^(\s*(?:---\n[\s\S]*?\n---\n)?\s*)#[ \t][^\n]*\n?/, "$1");
}

// wordsIn is roughly how many words a document has, not counting its code.
export function wordsIn(text: string): number {
  return text
    .replace(/```[\s\S]*?```/g, " ")
    .split(/\s+/)
    .filter((w) => /\w/.test(w)).length;
}
