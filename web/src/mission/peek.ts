import type { Headline, MissionDetail } from "../api";
import { prHref } from "../reviews/prs";
import { crewStatus, headlineStatus } from "../ui/statusOf";
import { briefGoal } from "./brief";
import { cardTone } from "./cardTone";
import { checkingTitle } from "./Checking/words";
import type { MissionPeekProps } from "./MissionPeek";

// peekOf is what a mission's peek is given (MissionPeek), worked out of
// the mission as ws gave it: its goal from its brief (or what was asked,
// before there's a brief), its cards without what they offer, its plan's
// steps, the crew that haven't ended, and its capsules with their pull
// requests.
export function peekOf(d: MissionDetail): MissionPeekProps {
  return {
    goal: (d.briefUnwritten ? "" : briefGoal(d.brief)) || d.request || undefined,
    stands: d.cards.map((c) => {
      const checking = c.checking as unknown as Headline["checking"];
      return {
        tone: cardTone[c.tone],
        icon: headlineStatus({ kind: c.kind, court: c.court, label: "" } as Headline).icon,
        title: checking ? checkingTitle(checking) : c.title,
        detail: checking ? undefined : c.detail,
        nothing: c.nothing,
      };
    }),
    steps: d.stages.map((s) => ({ n: s.n, title: s.title, state: s.state, skipped: s.skipped, hint: s.type ? `${s.type}: ${s.text}` : s.text })),
    crew: d.crew
      .filter((c) => !c.ended)
      .map((c) => ({
        id: c.id,
        status: crewStatus(c),
        what: c.model ? `${c.harness} on ${c.model}` : c.harness,
        capsule: c.capsule,
        to: `/missions/${encodeURIComponent(d.slug)}/crew/${encodeURIComponent(c.id)}`,
      })),
    capsules: d.capsules.map((c) => ({
      name: `${c.repo}/${c.name}`,
      gone: !c.exists,
      pr: c.pr && {
        name: `${c.repo}#${c.pr.number}`,
        to: prHref(c.repo, c.pr.number),
        status: c.pr.merged
          ? { icon: "check-circle", tone: "done", words: "Merged" }
          : c.pr.checks === "failure"
            ? { icon: "x-circle", tone: "danger", words: "Checks failing" }
            : c.pr.checks === "pending"
              ? { icon: "clock", tone: "waiting", words: "Checks running" }
              : { icon: "pr", tone: "quiet", words: "Open" },
      },
    })),
  };
}
