import { Time } from "../ui/Time";
import { ArtifactNote } from "./ArtifactNote";

type ArtifactDiscardedNoteProps = {
  at: string;
  by: string;
  reason?: string;
  // Its files have been deleted for good; otherwise until is the day its
  // folder's kept to.
  erased?: boolean;
  until: string;
};

// ArtifactDiscardedNote says a discarded artifact is off the list: when,
// by whom and why, and how long its folder's kept.
export function ArtifactDiscardedNote({ at, by, reason, erased, until }: ArtifactDiscardedNoteProps) {
  return (
    <ArtifactNote dim>
      Discarded <Time iso={at} as="relative" /> by {by}
      {reason ? `: ${reason}${/[.!?]$/.test(reason) ? "" : "."}` : "."} It's off the list, and takes no new versions.{" "}
      {erased ? "Its files have been deleted for good." : `Its folder is kept in .ws/artifacts/.discarded until ${until}.`}
    </ArtifactNote>
  );
}
