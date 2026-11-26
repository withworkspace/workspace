// What can be opened beside the page: a thing with a narrow form of its
// own, a pane. A tab beside is one of these, kept by its address in the
// web UI (the same address its own page has), so the tab and the page are
// one thing's.

export type PaneAt =
  | { kind: "conversation"; slug: string; id: string }
  | { kind: "ask"; thing: string }
  // A conversation that hasn't begun, about nothing: C, anywhere.
  | { kind: "new" }
  | { kind: "mission"; slug: string }
  // ref: the document as ws names it (repo:path, or repo/capsule:path).
  | { kind: "document"; ref: string }
  | { kind: "pr"; repo: string; number: number }
  // A review's comments, listed to go to: of is the review's address.
  | { kind: "comments"; of: string };

// The address a conversation has beside the page: its own page's.
export const talkAt = (id: string, slug = "") => (slug ? `/missions/${encodeURIComponent(slug)}/conversations/${encodeURIComponent(id)}` : `/conversations/${encodeURIComponent(id)}`);

// The address of a conversation that hasn't begun: one about a thing,
// open beside its page with nothing asked yet. It has no page of its own,
// and is there only while the thing's page is.
export const askAt = (thing: string) => `/ask/${encodeURIComponent(thing)}`;

// The address of a conversation you're about to begin, about nothing: open
// beside whatever page you're on, with nothing said yet. ws has no
// conversation for it until something is: then it's that conversation, in
// this tab's place. (No conversation is called new: they're w1, t3.)
export const newTalkAt = "/conversations/new";

// The address of a review's comments beside it: its own address, under
// /comments. It has no page of its own: the list is the review's, while
// it's open.
export const commentsAt = (review: string) => `/comments${review}`;

// A piece of an address as it was written, or as it is if it wasn't
// written as one.
function piece(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

// paneAt is the pane an address has, if what's there has one: a
// conversation (the workspace's, or about a mission), one that hasn't
// begun (about a thing, or about nothing), a mission, a document (a file
// in a repo, as it is on its default branch or, with ?in=, in a capsule)
// or a pull request. What else the address says (a mission's tab, a
// document being edited, a section) isn't the pane's.
export function paneAt(to: string): PaneAt | undefined {
  const path = to.split(/[?#]/)[0];
  if (path === newTalkAt) return { kind: "new" };
  if (path.startsWith("/comments/")) return { kind: "comments", of: path.slice("/comments".length) };
  let m = /^\/missions\/([^/]+)\/conversations\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "conversation", slug: piece(m[1]), id: piece(m[2]) };
  m = /^\/conversations\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "conversation", slug: "", id: piece(m[1]) };
  m = /^\/ask\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "ask", thing: piece(m[1]) };
  m = /^\/missions\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "mission", slug: piece(m[1]) };
  m = /^\/reviews\/([^/]+)\/(\d+)\/?$/.exec(path);
  if (m) return { kind: "pr", repo: piece(m[1]), number: Number(m[2]) };
  // /documents/local/<id> is a page nobody's filed: it has no pane yet.
  m = /^\/documents\/([^/]+)\/(.+)$/.exec(path);
  if (m && m[1] !== "local") {
    const capsule = new URLSearchParams(to.split("#")[0].split("?")[1] ?? "").get("in") ?? "";
    return { kind: "document", ref: `${piece(m[1])}${capsule ? `/${capsule}` : ""}:${m[2].split("/").map(piece).join("/")}` };
  }
  return undefined;
}

// peekAt is the tab a link opens when it's peeked at (z): the address of
// the thing it goes to, without what the link says of where in it (a
// mission's tab, a section, a document to edit), so two links to one
// thing are one tab. Nothing, for a link that leaves ws or goes to
// something with no pane.
export function peekAt(href: string, origin: string): string | undefined {
  let url: URL;
  try {
    url = new URL(href, origin);
  } catch {
    return undefined;
  }
  if (url.origin !== origin) return undefined;
  const at = paneAt(url.pathname + url.search);
  // One that hasn't begun isn't somewhere a link goes.
  if (!at || at.kind === "ask" || at.kind === "new") return undefined;
  const path = url.pathname.replace(/\/$/, "");
  if (at.kind !== "document") return path;
  const capsule = url.searchParams.get("in");
  return capsule ? `${path}?in=${encodeURIComponent(capsule)}` : path;
}
