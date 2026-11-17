// Where a document's links and images lead: a file beside it in the same
// repo, a heading in it, or somewhere else altogether.

// Place is where a document being read is: the worktree it's in (api:
// where its files are read) and its path there. to is the page that opens
// another of the worktree's files.
export interface Place {
  api: string;
  path: string;
  to: (path: string) => string;
}

export type Target =
  | { kind: "external"; href: string } // an address of its own (https:, mailto:)
  | { kind: "heading"; id: string } // a heading in this document
  | { kind: "file"; path: string; hash: string } // a file in the worktree
  | { kind: "outside" }; // a path that leaves the worktree

const decode = (s: string) => {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
};

// resolveLink reads a link as GitHub does for a file in a repo: against
// the folder the document's in, or the repo's root for one that starts
// with a slash.
export function resolveLink(docPath: string, href: string): Target {
  if (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.startsWith("//")) return { kind: "external", href };
  const cut = href.indexOf("#");
  const hash = cut < 0 ? "" : href.slice(cut + 1);
  const file = (cut < 0 ? href : href.slice(0, cut)).replace(/\?.*$/, "");
  if (!file) return { kind: "heading", id: decode(hash) };
  const out = file.startsWith("/") ? [] : docPath.split("/").slice(0, -1);
  for (const part of decode(file).split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      if (!out.length) return { kind: "outside" };
      out.pop();
    } else out.push(part);
  }
  return { kind: "file", path: out.join("/"), hash };
}

// slugger names headings as GitHub does, for links to them: lower case,
// letters, digits, hyphens and underscores kept, spaces to hyphens, and a
// number after one that's been used (-1, -2).
export function slugger(): (text: string) => string {
  const used = new Set<string>();
  return (text) => {
    const base = text
      .trim()
      .toLowerCase()
      .replace(/[^\p{L}\p{M}\p{N}\p{Pc}\- ]/gu, "")
      .replace(/ /g, "-");
    let slug = base;
    for (let n = 1; used.has(slug); n++) slug = `${base}-${n}`;
    used.add(slug);
    return slug;
  };
}
