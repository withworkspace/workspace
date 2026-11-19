import { getPR, type PR } from "../api";

// A pull request, read once while the page is open: for its card, and for
// how it stands where it's pinned (shell/focus.tsx).
const prs = new Map<string, Promise<PR>>();

export const prOf = (repo: string, number: number) => {
  const key = `${repo}#${number}`;
  let got = prs.get(key);
  if (!got) {
    got = getPR(repo, number).then((d) => d.pr);
    prs.set(key, got);
    got.catch(() => prs.delete(key));
  }
  return got;
};
