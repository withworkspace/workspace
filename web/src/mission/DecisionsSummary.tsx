import { useMemo } from "react";
import type { MissionDetail } from "../api";
import { MoreLink } from "../ui/MoreLink";
import css from "./DecisionsSummary.module.css";
import { decisionsWords, groupDecisions } from "./decisions";
import { MissionDecision } from "./MissionDecision";

// How many of yours a mission's Summary shows.
const shown = 3;

// DecisionsSummary is a mission's decisions on its Summary: yours, the
// latest few (those you decided outright before what you said when
// asked), as they're the ones that stand over the plan, and the way to
// all of them, saying how many there are and whose.
export function DecisionsSummary({ d, all }: { d: MissionDetail; all: () => void }) {
  const g = useMemo(() => groupDecisions(d.decisions, d.stages), [d.decisions, d.stages]);
  return (
    <div className={css.summary}>
      {[...g.yours, ...g.answered].slice(0, shown).map((x) => (
        <MissionDecision key={x.id} d={x} yours dense />
      ))}
      <MoreLink onClick={all}>{decisionsWords(g)} in Decisions</MoreLink>
    </div>
  );
}
