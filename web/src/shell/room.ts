// wantsRoom says whether a page wants the room: a mission (and what's under
// it: its reviews, files and crew), a pull request's review, a repo's files,
// a running artifact, an app (a run of the workspace's, framed), and a
// space of documents (its list beside the
// document picked; not Documents, nor a document read). There the sidebar
// folds to a rail, which opens over the page on hover.
export function wantsRoom(pathname: string): boolean {
  return pathname.startsWith("/missions/") || pathname.startsWith("/apps/") || pathname.includes("/files/") || pathname.startsWith("/reviews/") || pathname.startsWith("/artifacts/") || isSpace(pathname);
}

// isSpace is a space's own page, /documents/<repo> (a local page,
// /documents/local/<id>, and a document, /documents/<repo>/<path>, aren't).
const isSpace = (pathname: string) => /^\/documents\/[^/]+\/?$/.test(pathname) && pathname !== "/documents/local" && pathname !== "/documents/local/";
