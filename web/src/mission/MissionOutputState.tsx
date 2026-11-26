import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { capital } from "../ui/strings";
import { type Tone } from "../ui/tone";

const stateTone: Record<string, Tone> = {
  draft: "quiet",
  private: "quiet",
  open: "waiting",
  "in review": "waiting",
  approved: "done",
  landed: "done",
  merged: "done",
  missing: "danger",
  // An artifact's: live, promoted, discarded.
  live: "progress",
  promoted: "done",
  discarded: "quiet",
};

// MissionOutputState is where one of a mission's outputs stands (draft,
// in review, landed; an artifact's live, promoted, discarded), as a chip
// in its tone.
export function MissionOutputState({ state }: { state: string }) {
  return (
    <Chip tone={stateTone[state] ?? "quiet"}>
      {state === "private" && <Icon name="lock" size={11} />}
      {capital(state)}
    </Chip>
  );
}
