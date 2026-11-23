// What can be opened beside the page: a thing with a narrow form of its
// own, a pane. A tab beside is one of these, kept by its address in the
// web UI (the same address its own page has), so the tab and the page are
// one thing's.

export type PaneAt = { kind: "conversation"; slug: string; id: string } | { kind: "ask"; thing: string };

// The address a conversation has beside the page: its own page's.
export const talkAt = (id: string, slug = "") => (slug ? `/missions/${encodeURIComponent(slug)}/conversations/${encodeURIComponent(id)}` : `/conversations/${encodeURIComponent(id)}`);

// The address of a conversation that hasn't begun: one about a thing,
// open beside its page with nothing asked yet. It has no page of its own,
// and is there only while the thing's page is.
export const askAt = (thing: string) => `/ask/${encodeURIComponent(thing)}`;

// paneAt is the pane an address has, if what's there has one: a
// conversation (the workspace's, or about a mission), or one about a
// thing that hasn't begun.
export function paneAt(to: string): PaneAt | undefined {
  const path = to.split(/[?#]/)[0];
  let m = /^\/missions\/([^/]+)\/conversations\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "conversation", slug: decodeURIComponent(m[1]), id: decodeURIComponent(m[2]) };
  m = /^\/conversations\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "conversation", slug: "", id: decodeURIComponent(m[1]) };
  m = /^\/ask\/([^/]+)\/?$/.exec(path);
  if (m) return { kind: "ask", thing: decodeURIComponent(m[1]) };
  return undefined;
}
