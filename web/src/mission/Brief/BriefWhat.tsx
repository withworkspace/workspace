import type { Stage, TalkProposal } from "../../api";
import { Time } from "../../ui/Time";
import { MissionBriefWhat, missionBriefWhatQuietClass } from "./MissionBriefWhat";
import { proposedName, proposedTitle } from "./words";
import { Text } from "../../ui/Text";

type BriefWhatProps = {
  // It's ended: as it was written.
  ended: boolean;
  briefing: boolean;
  // What's proposed for the documents, and which draft of the brief it is.
  proposal?: TalkProposal;
  draft: number;
  // Your edits aren't saved.
  changed: boolean;
  // Its brief and plan are written: confirming them starts it.
  confirmable: boolean;
  // The step the mission's at, of how many.
  now?: Stage;
  steps: number;
};

// BriefWhat says where the brief and plan stand, in the tab's bar: a
// draft or an edit proposed, your edits not saved, written and to
// confirm, or as written, with the step the mission's at.
export function BriefWhat({ ended, briefing, proposal: p, draft, changed, confirmable, now, steps }: BriefWhatProps) {
  if (ended) return <MissionBriefWhat icon="check-circle">Ended: as written</MissionBriefWhat>;
  if (p) {
    const said = { briefing, kind: p.kind, draft, plan: !!p.plan };
    return (
      <MissionBriefWhat icon={p.kind === "plan" ? "list-ordered" : "pencil"} tone="attention" title={proposedTitle(said)}>
        {proposedName(said)}{" "}
        <Text dim className={missionBriefWhatQuietClass}>
          · <Time iso={p.at} as="relative" />
        </Text>
      </MissionBriefWhat>
    );
  }
  if (changed)
    return (
      <MissionBriefWhat icon="pencil" tone="attention">
        Unsaved edits
      </MissionBriefWhat>
    );
  if (confirmable)
    return (
      <MissionBriefWhat icon="check-circle" title="Confirm to start the mission">
        Written
      </MissionBriefWhat>
    );
  if (now)
    return (
      <MissionBriefWhat icon="circle-dot">
        As written{" "}
        <Text dim className={missionBriefWhatQuietClass}>
          · at step {now.n} of {steps}
        </Text>
      </MissionBriefWhat>
    );
  return <MissionBriefWhat icon="file">{briefing ? "To brief" : "As written"}</MissionBriefWhat>;
}
