import { GROUND, type Known, type PageRef, refName, thingAtPath } from "../refs/refs";

// What's pinned is named as ws names it: what a ref, or the page at an
// address, is pinned as.

// pinNameOf is what a ref is pinned as, or undefined for what isn't
// pinned: a file or folder on the default branch rather than its lines,
// an artifact rather than one version. A capsule's file, or a repo, isn't
// something to focus on.
export function pinNameOf(r: PageRef): string | undefined {
  switch (r.kind) {
    case "file":
      return r.worktree === GROUND
        ? refName({
            kind: "file",
            repo: r.repo,
            worktree: GROUND,
            path: r.path,
          })
        : undefined;
    case "dir":
      return r.worktree === GROUND ? refName(r) : undefined;
    case "artifact":
      return r.id;
    case "repo":
      return undefined;
  }
  return refName(r);
}

// pageRefAt is what the page at an address is, of what's pinned: a
// mission (any of its pages), a conversation, a pull request, a page, an
// artifact, a document on its space's default branch, a plugin's thing.
export function pageRefAt(path: string, search: string, known?: Known): PageRef | undefined {
  const seg = path
    .split("/")
    .slice(1)
    .map((s) => {
      try {
        return decodeURIComponent(s);
      } catch {
        return s;
      }
    });
  const page = (id?: string): PageRef | undefined => (id && /^p[1-9]\d*$/.test(id) ? { kind: "page", id } : undefined);
  switch (seg[0]) {
    case "missions":
      return seg[1] ? { kind: "mission", slug: seg[1] } : undefined;
    case "conversations":
      return seg[1] ? { kind: "talk", id: seg[1] } : undefined;
    case "reviews":
      return seg[1] && /^\d+$/.test(seg[2] ?? "") ? { kind: "pr", repo: seg[1], number: Number(seg[2]) } : undefined;
    case "pages":
      return page(seg[1]);
    case "artifacts":
      return seg[1] && seg[1] !== "compare" ? { kind: "artifact", id: seg[1] } : undefined;
    case "documents":
      if (seg[1] === "local") return page(seg[2]);
      return seg[1] && seg.length > 2 && !new URLSearchParams(search).has("in") ? { kind: "file", repo: seg[1], worktree: GROUND, path: seg.slice(2).join("/") } : undefined;
    case "x":
      return known ? thingAtPath(path, known) : undefined;
  }
  return undefined;
}
