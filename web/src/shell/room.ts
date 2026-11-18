// wantsRoom says whether a page wants the room: a mission (and what's under
// it: its reviews, files and crew), a pull request's review, a repo's files
// and a running artifact. There the sidebar folds to a rail, which opens
// over the page on hover.
export function wantsRoom(pathname: string): boolean {
  return pathname.startsWith("/missions/") || pathname.includes("/files/") || pathname.startsWith("/reviews/") || pathname.startsWith("/artifacts/");
}
