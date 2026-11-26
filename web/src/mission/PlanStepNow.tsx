import { Fragment } from "react";
import type { MissionDetail, Stage } from "../api";
import { Inline } from "../ui/Inline";
import { crewStatus } from "../ui/statusOf";
import { stamp } from "../ui/when";
import { CrewBadge } from "./CrewBadge";
import { MissionPlanCheckList } from "./MissionPlanCheckList";
import { MissionPlanLine } from "./MissionPlanLine";
import { MissionPlanNow } from "./MissionPlanNow";
import { PlanWaitLine } from "./PlanWaitLine";
import { StepLimit } from "./StepLimit";
import { StepRuns } from "./StepRuns";
import { checksUnmet, checksWords, stepKind, stepYours, waitHow } from "./planRows";

// PlanStepNow is the step the plan is on, said in place: its kind and where,
// what it makes, who's on it and what they're doing, its clock, and each
// check with how it stands.
export function PlanStepNow({ s, d }: { s: Stage; d?: MissionDetail }) {
  const kind = stepKind(s);
  const checks = s.checks ?? [];
  const how = waitHow(s);
  return (
    <MissionPlanNow>
      {kind && (
        <MissionPlanLine icon={kind[0]}>
          <strong>{kind[1]}</strong>
          {how ? s.on && s.on !== "you" && <Inline text={`on ${s.on}`} /> : s.whoSays && <Inline text={s.whoSays} />}
        </MissionPlanLine>
      )}
      {how && <PlanWaitLine how={how} />}
      {s.makes && (
        <MissionPlanLine words>
          Makes <Inline text={s.makes} />
        </MissionPlanLine>
      )}
      {!!s.crew?.length && (
        <MissionPlanLine>
          {s.crew.map((id) => {
            const c = d?.crew.find((x) => x.id === id);
            return (
              <Fragment key={id}>
                <CrewBadge id={id} />
                {c && <span>{crewStatus(c).words.toLowerCase()}</span>}
              </Fragment>
            );
          })}
        </MissionPlanLine>
      )}
      {(stepYours(s) || s.held) && (
        <MissionPlanLine icon="user" yours>
          {how ? "Yours to do" : "Yours: only you can pass"}
        </MissionPlanLine>
      )}
      {!!s.carryOn && <MissionPlanLine icon="reply">Carries on from step {s.carryOn}</MissionPlanLine>}
      {!!s.attempts && <MissionPlanLine icon="rotate">Attempt {s.attempts + 1}</MissionPlanLine>}
      {s.limit && (
        <MissionPlanLine>
          <StepLimit s={s} />
        </MissionPlanLine>
      )}
      {s.onlyIf && (
        <MissionPlanLine icon="question">
          <span>
            Only if <Inline text={s.onlyIf} />
          </span>
        </MissionPlanLine>
      )}
      {s.inEach && (
        <MissionPlanLine>
          <StepRuns s={s} />
        </MissionPlanLine>
      )}
      {checks.length > 0 && (
        <>
          <MissionPlanLine icon={checksUnmet(s) ? "alert" : undefined} warn={checksUnmet(s)}>
            {checksWords(s, stamp)}
          </MissionPlanLine>
          <MissionPlanCheckList checks={checks} />
        </>
      )}
    </MissionPlanNow>
  );
}
