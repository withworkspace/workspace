import { useContext, useMemo, useRef, useState } from "react";
import type { Stage } from "../api";
import { FloatingCard } from "../ui/FloatingCard";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanFold } from "./MissionPlanFold";
import { MissionPlanPip } from "./MissionPlanPip";
import { useHoverDelay } from "../ui/useHoverDelay";
import { RefContext } from "../refs/context";
import { MissionData } from "./MissionData";
import { missionStepCardClass } from "./MissionStepCard";
import { PlanStep } from "./PlanStep";
import { PlanStepCard } from "./PlanStepCard";
import { foldNote, passedUnmet, planWindow, stepTimes, stepYours } from "./planRows";

// PlanList is the mission's plan (or its brief's stages) in its details: a
// row a step, with how it stands, its number and name, and its kind at
// the row's end (in the accent when it's yours). The step it's on is opened
// where that says more than the row does. A long plan shows the steps
// round that one, and folds the rest into a row where they are, each a
// pip; the row opens them. What there isn't room for in a row is its card,
// beside the list.
//
// brief: in a brief's outline, where a row is pressed to go to the step in
// the plan, and the step it's on isn't said in place.
export function PlanList({ stages, brief }: { stages: Stage[]; brief?: boolean }) {
  const d = useContext(MissionData);
  const [open, setOpen] = useState<{ earlier?: boolean; later?: boolean }>({});
  const { earlier, shown, later } = useMemo(() => planWindow(stages), [stages]);
  const times = useMemo(() => stepTimes(d?.events ?? []), [d?.events]);

  // One card for the list: of the step, or the pip, the pointer's on.
  const list = useRef<HTMLOListElement>(null);
  const hover = useHoverDelay<number>();
  const els = useRef(new Map<number, HTMLElement>());
  const card = (s: Stage) => ({
    ref: (el: HTMLElement | null) => {
      if (el) els.current.set(s.n, el);
      else els.current.delete(s.n);
    },
    // The opened step's own block has its crew's cards: not this one too.
    onMouseOver: (e: React.MouseEvent) => ((e.target as Element).closest("[data-plan-now]") ? hover.out() : hover.over(s.n)),
    onMouseLeave: hover.out,
    onFocus: () => hover.over(s.n),
    onBlur: hover.out,
  });
  const carded = hover.shown === undefined ? undefined : stages.find((s) => s.n === hover.shown);
  const anchor = hover.shown === undefined ? undefined : els.current.get(hover.shown);

  const rows = (run: Stage[]) => run.map((s) => <PlanStep key={s.n} s={s} d={d} card={card(s)} brief={brief} />);
  const fold = (run: Stage[], side: "earlier" | "later") => {
    if (!run.length) return null;
    const toggle = () => {
      hover.clear();
      setOpen((o) => ({ ...o, [side]: !o[side] }));
    };
    const row = (
      <MissionPlanFold key={`fold-${side}`} side={side} count={run.length} open={open[side]} onToggle={toggle} note={foldNote(run, side)}>
        {run.map((s) => (
          <MissionPlanPip key={s.n} state={s.state} warn={passedUnmet(s)} yours={s.state === "todo" && stepYours(s)} wait={s.type === "Wait"} {...card(s)} />
        ))}
      </MissionPlanFold>
    );
    if (!open[side]) return row;
    return side === "earlier" ? [row, ...rows(run)] : [...rows(run), row];
  };

  return (
    <>
      <MissionPlan rows ref={list}>
        {fold(earlier, "earlier")}
        {rows(shown)}
        {fold(later, "later")}
      </MissionPlan>
      {carded && anchor && (
        <FloatingCard anchor={anchor} beside={list.current} className={missionStepCardClass()} onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>
            <PlanStepCard s={carded} of={stages.length} d={d} time={times.get(carded.n)} />
          </RefContext.Provider>
        </FloatingCard>
      )}
    </>
  );
}
