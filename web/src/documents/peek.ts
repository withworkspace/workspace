import { type ArtifactContent, GROUND } from "../api";
import { withoutTitle, wordsIn } from "./happening";

// titleOf is what a document calls itself: its first heading if it opens
// with one (after any front matter), else its file's name.
export function titleOf(text: string, path: string): string {
  const m = /^(?:\s*---\n[\s\S]*?\n---\n)?\s*#[ \t]+([^\n]+)/.exec(text);
  return m?.[1].trim().replace(/\s+#+$/, "") || path.split("/").pop() || path;
}

// A document as its peek has it (DocumentPeek), worked out of the document
// as ws gave it: what it's called, where it is, how long it is, and its
// text without the heading that's its title.
export interface DocumentPeekOf {
  title: string;
  repo: string;
  capsule?: string;
  path: string;
  words: number;
  text: string;
  mission?: { title: string; to: string };
}

// docPeekOf works that out. ref is the document as it was asked for
// (repo:path, or repo/capsule:path), which says where it is when ws
// didn't; label is what the workspace calls a repo.
export function docPeekOf(a: ArtifactContent, ref: string, label: (repo: string) => string = (r) => r): DocumentPeekOf {
  const at = ref.indexOf(":");
  const [repo, capsule] = ref.slice(0, at).split("/");
  const where = a.in ?? { repo, worktree: capsule || GROUND, path: ref.slice(at + 1) };
  const text = withoutTitle(a.text);
  return {
    title: titleOf(a.text, where.path),
    repo: label(where.repo),
    capsule: where.worktree === GROUND ? undefined : where.worktree,
    path: where.path,
    words: wordsIn(text),
    text,
    mission: a.mission && { title: a.mission.title, to: `/missions/${encodeURIComponent(a.mission.slug)}/artifact?${new URLSearchParams({ ref: a.mission.ref })}` },
  };
}
