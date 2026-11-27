import { useState } from "react";
import { act, type MissionDetail, type Stage } from "../api";
import type { Reload } from "../load";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { CrewBadge } from "./CrewBadge";
import { type Level, MissionPlanLevel } from "./MissionPlanLevel";
import { MissionPlanNote } from "./MissionPlanNote";
import { MissionPlanStep } from "./MissionPlanStep";
import { PlanStepNow } from "./PlanStepNow";
import { StepChecks } from "./StepChecks";
import { StepLimit } from "./StepLimit";
import { checksUnmet, stepKind, stepYours, waitHow, worthOpening } from "./planRows";

// What a row of the plan is given so its card (PlanStepCard) opens beside
// it: where it is, and the pointer and the focus coming and going.
export type PlanCardProps = {
  ref: (el: HTMLElement | null) => void;
  onMouseOver: (e: React.MouseEvent) => void;
  onMouseLeave: () => void;
  onFocus: () => void;
  onBlur: () => void;
};

// PlanStep is a step of the plan in its details, a row: nothing but its
// name and kind, unless it needs you to look (the current step's crew and
// whether it's yours, checks not met, time run out), or it's the one the
// plan is on and worth opening.
export function PlanStep({ s, d, card, brief, reload }: { s: Stage; d?: MissionDetail; card: PlanCardProps; brief?: boolean; reload?: Reload }) {
  const [busy, setBusy] = useState(false);
  const kind = stepKind(s);
  const how = waitHow(s);
  const current = s.state === "current";
  const opened = current && worthOpening(s);
  const yours = current && (stepYours(s) || s.held);
  const late = !!s.outOfTime && s.state !== "accepted";
  const unmet = checksUnmet(s);
  // A Work step's level, which you can change while nobody's begun it.
  const level = s.level as Level | undefined;
  const changeable = !!d && !!reload && !brief && s.state !== "accepted" && !s.crew?.length;
  const choose = (to: Level) => {
    if (!d || busy) return;
    setBusy(true);
    act(d.slug, "level", { step: s.n, text: to })
      .then(async (r) => {
        toast(r.message, { tone: "done" });
        await reload?.();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  return (
    <MissionPlanStep
      row
      n={s.n}
      title={s.title}
      state={s.state}
      yours={stepYours(s) && s.state !== "accepted"}
      skipped={s.skipped}
      opened={opened}
      pressable={brief}
      level={level && s.state !== "accepted" && <MissionPlanLevel level={level} on={d?.levelOn} onChoose={changeable ? choose : undefined} busy={busy} />}
      kind={kind && { icon: kind[0], says: how && s.state !== "accepted" ? `${kind[1]} · ${how.word.toLowerCase()}: ${how.says}` : kind[1] }}
      under={opened && !brief && <PlanStepNow s={s} d={d} />}
      tabIndex={0}
      {...card}
    >
      {!opened && (yours || late || unmet || (current && !!s.crew?.length)) && (
        <>
          {yours && (
            <MissionPlanNote icon="user" yours title="Yours: only you can pass">
              yours
            </MissionPlanNote>
          )}
          {late && <StepLimit s={s} />}
          {unmet && <StepChecks s={s} />}
          {current && s.crew?.map((c) => <CrewBadge key={c} id={c} />)}
        </>
      )}
    </MissionPlanStep>
  );
}
