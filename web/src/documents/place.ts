import type { DocKind, PageInfo, Space } from "../api";

// Where a page is listed, in Documents:
//
//   tied to a file in a space, with something to save  → that space, as your draft of it
//   tied to one with nothing to save                   → nowhere: the document is listed
//   started as a kind whose path is in a space, unfiled → that space, as your draft
//   started to be filed in a space's folder, unfiled   → that space, as your draft
//   filed in a space                                   → that space while it's in flight, then the document is listed
//   anything else                                      → Local

// Placed is a page where it's listed: Local, or a space (repo) as a draft.
export interface Placed {
  page: PageInfo;
  repo?: string; // the space it's a draft in; none for Local
  why?: "unsaved" | "kind" | "to" | "filed"; // what makes it a draft there
  path?: string; // the document it's of, in the space
}

// refPlace is a document's ref as its repo and its path: repo:path, or
// repo/capsule:path.
export function refPlace(ref: string): { repo: string; path: string; capsule?: string } | undefined {
  const at = ref.indexOf(":");
  if (at < 0) return undefined;
  const [repo, capsule] = ref.slice(0, at).split("/");
  return { repo, path: ref.slice(at + 1), capsule };
}

// spaceOf is the space a document in repo at path is in, if it's in one.
export function spaceOf(spaces: Space[], repo: string, path: string): Space | undefined {
  return spaces.find((s) => s.repo === repo && path.startsWith(s.folder ?? "") && /\.(md|markdown|mdx)$/i.test(path));
}

// placePages is where each page is listed, those listed nowhere left out.
export function placePages(pages: PageInfo[], spaces: Space[], kinds: DocKind[]): Placed[] {
  const out: Placed[] = [];
  for (const page of pages) {
    if (page.filed) {
      const at = refPlace(page.filed.ref);
      const s = at && spaceOf(spaces, at.repo, at.path);
      if (s && at) {
        if (!page.filed.landed) out.push({ page, repo: s.repo, why: "filed", path: at.path });
        continue;
      }
    }
    if (page.source) {
      const at = refPlace(page.source.ref);
      const s = at && spaceOf(spaces, at.repo, at.path);
      if (s && at) {
        if (page.unsaved) out.push({ page, repo: s.repo, why: "unsaved", path: at.path });
        continue;
      }
      out.push({ page });
      continue;
    }
    if (page.to && !page.filed) {
      const at = refPlace(page.to);
      const s = at && spaces.find((x) => x.repo === at.repo && at.path.startsWith(x.folder ?? ""));
      if (s) {
        out.push({ page, repo: s.repo, why: "to", path: at.path });
        continue;
      }
    }
    if (page.kind && !page.filed) {
      const k = kinds.find((x) => x.name === page.kind);
      const at = k && refPlace(k.where);
      const s = at && spaces.find((x) => x.repo === at.repo && at.path.startsWith(x.folder ?? ""));
      if (s) {
        out.push({ page, repo: s.repo, why: "kind" });
        continue;
      }
    }
    out.push({ page });
  }
  return out;
}
