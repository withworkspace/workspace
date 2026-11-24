import { PaneSection } from "../panes/PaneSection";
import { MissionPeekCapsule, type MissionPeekCapsuleInfo } from "./MissionPeekCapsule";
import { MissionPeekCrew, type MissionPeekCrewInfo } from "./MissionPeekCrew";
import { MissionPeekStand, type MissionPeekStandInfo } from "./MissionPeekStand";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanStep } from "./MissionPlanStep";
import css from "./MissionPeek.module.css";

export type MissionPeekStepInfo = {
  n: number;
  title: string;
  state: "accepted" | "current" | "todo";
  skipped?: boolean;
  // The whole step, said when it's pointed at.
  hint?: string;
};

export type MissionPeekProps = {
  // What it's for, in a line: its brief's goal, or what was asked for.
  goal?: string;
  // What it stands on, the first the one that matters most.
  stands: MissionPeekStandInfo[];
  steps: MissionPeekStepInfo[];
  // Crew that haven't ended.
  crew: MissionPeekCrewInfo[];
  capsules: MissionPeekCapsuleInfo[];
};

// MissionPeek is a mission as it's read beside another page: what it's
// for, where it stands and whose move it is, how far through its plan it
// is, who's at it now, and where its work is. It's for looking, so
// nothing here moves the mission on: that's its own page's, one press
// away at the pane's head. What it's given is worked out of the mission
// by peekOf (peek.ts).
export function MissionPeek({ goal, stands, steps, crew, capsules }: MissionPeekProps) {
  return (
    <div className={css.peek}>
      {goal && <p className={css.goal}>{goal}</p>}
      {stands.length > 0 && (
        <div className={css.stands}>
          {stands.map((s, i) => (
            <MissionPeekStand key={`${i}-${s.title}`} {...s} lead={i === 0} />
          ))}
        </div>
      )}
      {steps.length > 0 && (
        <PaneSection title="Plan" count={`${steps.filter((s) => s.state === "accepted").length} of ${steps.length}`}>
          <MissionPlan>
            {steps.map((s) => (
              <MissionPlanStep key={s.n} n={s.n} title={s.title} state={s.state} skipped={s.skipped} hint={s.hint} />
            ))}
          </MissionPlan>
        </PaneSection>
      )}
      {crew.length > 0 && (
        <PaneSection title="Crew" count={crew.length}>
          <ul className={css.list}>
            {crew.map((c) => (
              <MissionPeekCrew key={c.id} {...c} />
            ))}
          </ul>
        </PaneSection>
      )}
      {capsules.length > 0 && (
        <PaneSection title="Capsules" count={capsules.length}>
          <ul className={css.list}>
            {capsules.map((c) => (
              <MissionPeekCapsule key={c.name} {...c} />
            ))}
          </ul>
        </PaneSection>
      )}
    </div>
  );
}
