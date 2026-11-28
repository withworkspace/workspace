import type { PRState, ReviewRow } from "../api";
import type { IconName } from "../ui/Icon";
import type { StatusInfo } from "../ui/Status";
import { capital } from "../ui/strings";
import type { Tone } from "../ui/tone";
import { rowAfter } from "./after";
import { useReviewsMemory } from "./memory";

// What else reads of the Reviews page's pull requests, kept apart from the
// page so reading them doesn't bring the page's code (shell/lazyPage.tsx).

const stateInfo: Record<PRState, { icon: IconName; tone: Tone }> = {
  "needs-review": { icon: "circle-dot", tone: "attention" },
  "back-with-you": { icon: "reply", tone: "attention" },
  "waiting-on-them": { icon: "clock", tone: "waiting" },
  approved: { icon: "check-circle", tone: "done" },
  others: { icon: "circle", tone: "quiet" },
  drafts: { icon: "circle-dashed", tone: "quiet" },
  automated: { icon: "sparkle", tone: "quiet" },
  returned: { icon: "alert", tone: "attention" },
  ready: { icon: "check-circle", tone: "attention" },
  "waiting-review": { icon: "clock", tone: "waiting" },
};

// prStatus is where a pull request stands with you.
export function prStatus(p: { state: PRState; words: string }): StatusInfo {
  return { ...(stateInfo[p.state] ?? { icon: "circle", tone: "quiet" }), words: capital(p.words) };
}

// prHref is a pull request's review here.
export const prHref = (repo: string, number: number) => `/reviews/${encodeURIComponent(repo)}/${number}`;

// reviewed moves the list's selection on from a pull request whose review
// has just been posted, so coming back to the list is at the next one.
export function reviewed(repo: string, number: number) {
  const { list, openSections } = useReviewsMemory.getState();
  const rows = (list?.sections ?? []).filter((s) => !s.folded || openSections[s.key]).flatMap((s) => s.rows);
  const next = rowAfter(rows, repo, number);
  if (next) useReviewsMemory.setState({ selected: next });
}

// rowTarget is where a row opens: a teammate's, its review here (a stack
// at the lowest layer that needs you); yours, its mission, which already
// says what moves it on, or with none, GitHub, where it's merged.
export function rowTarget(r: ReviewRow): { to?: string; href?: string } {
  const pr = r.pr ?? r.stack!.layers.find((l) => l.number === r.stack!.open) ?? r.stack!.layers[0];
  if (pr.mine) {
    const mission = r.pr ? r.pr.mission : r.stack!.layers.find((l) => l.mission)?.mission;
    return mission ? { to: `/missions/${mission.slug}` } : { href: pr.url };
  }
  return { to: prHref(pr.repo, pr.number) };
}
