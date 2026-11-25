import type { ReactNode } from "react";
import type { Stage } from "../api";
import { Icon } from "../ui/Icon";
import { CrewBadge } from "./CrewBadge";
import { MissionPlanNote } from "./MissionPlanNote";
import { StepChecks } from "./StepChecks";
import { StepLimit } from "./StepLimit";
import { StepRuns } from "./StepRuns";
import { StepWho } from "./StepWho";

// stepNotes is what's to be known of a step, under its name, or nothing
// where there's nothing to say.
export function stepNotes(s: Stage): ReactNode {
  if (
    !(
      s.crew?.length ||
      (s.checkIn && s.state !== "accepted") ||
      (s.who && s.who !== "crew") ||
      s.opensPR ||
      s.checks?.length ||
      s.limit ||
      s.carryOn ||
      s.onlyIf ||
      s.inEach ||
      s.skipped ||
      s.attempts
    )
  )
    return null;
  return (
    <>
      {s.who && s.who !== "crew" && <StepWho s={s} />}
      {s.type === "Verify" && (
        <MissionPlanNote icon="list-checks" title="A verifier's: crew who didn't do the work show it by using it, and hand over a bundle">
          verifier
        </MissionPlanNote>
      )}
      {s.opensPR && s.who !== "ws" && (
        <MissionPlanNote icon="pr" title="It makes a pull request: its crew open it through ws, which holds the guarantees about it">
          opens a pull request
        </MissionPlanNote>
      )}
      {s.skipped ? (
        <MissionPlanNote icon="minus" title={s.why ? `Skipped: ${s.why}` : "Skipped"}>
          skipped
        </MissionPlanNote>
      ) : s.onlyIf ? (
        <MissionPlanNote icon="question" yours={s.held} title={`Only if ${s.onlyIf}: the orchestrator starts it or skips it`}>
          only if
        </MissionPlanNote>
      ) : null}
      {s.inEach ? <StepRuns s={s} /> : null}
      {s.attempts ? (
        <MissionPlanNote icon="rotate" title={`Started again ${s.attempts} ${s.attempts === 1 ? "time" : "times"}`}>
          attempt {s.attempts + 1}
        </MissionPlanNote>
      ) : null}
      {s.carryOn ? (
        <MissionPlanNote icon="reply" title={`The crew who did step ${s.carryOn} carry on with this one`}>
          carries on from {s.carryOn}
        </MissionPlanNote>
      ) : null}
      {s.limit && s.state !== "accepted" && <StepLimit s={s} />}
      {s.checks?.length ? <StepChecks s={s} /> : null}
      {s.crew?.map((c) => (
        <CrewBadge key={c} id={c} />
      ))}
      {s.checkIn && s.state !== "accepted" && (
        <span className="dim" title="The orchestrator stops for you after this stage">
          <Icon name="pause" size={11} /> check in after
        </span>
      )}
    </>
  );
}
