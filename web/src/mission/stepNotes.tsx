import type { ReactNode } from "react";
import type { Stage } from "../api";
import { Icon } from "../ui/Icon";
import { CrewBadge } from "./CrewBadge";
import { MissionPlanNote } from "./MissionPlanNote";
import { StepChecks } from "./StepChecks";
import { StepLimit } from "./StepLimit";
import { StepRuns } from "./StepRuns";
import { StepWho } from "./StepWho";
import { Text } from "../ui/Text";

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
        <MissionPlanNote icon="list-checks" title="Verifier: other crew try the work and hand over a bundle">
          verifier
        </MissionPlanNote>
      )}
      {s.opensPR && s.who !== "ws" && (
        <MissionPlanNote icon="pr" title="Crew open a pull request through Workspace">
          opens a pull request
        </MissionPlanNote>
      )}
      {s.skipped ? (
        <MissionPlanNote icon="minus" title={s.why ? `Skipped: ${s.why}` : "Skipped"}>
          skipped
        </MissionPlanNote>
      ) : s.onlyIf ? (
        <MissionPlanNote icon="question" yours={s.held} title={`Only if ${s.onlyIf}: otherwise skipped`}>
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
        <MissionPlanNote icon="reply" title={`Same crew as step ${s.carryOn}`}>
          carries on from {s.carryOn}
        </MissionPlanNote>
      ) : null}
      {s.limit && s.state !== "accepted" && <StepLimit s={s} />}
      {s.checks?.length ? <StepChecks s={s} /> : null}
      {s.crew?.map((c) => (
        <CrewBadge key={c} id={c} />
      ))}
      {s.checkIn && s.state !== "accepted" && (
        <Text dim title="Orchestrator pauses for you after this stage">
          <Icon name="pause" size={11} /> check in after
        </Text>
      )}
    </>
  );
}
