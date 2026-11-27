import { Inline } from "../ui/Inline";
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
// by whom and why (as it was written: markdown, its names drawn as refs),
// and how long its folder's kept.
export function ArtifactDiscardedNote({ at, by, reason, erased, until }: ArtifactDiscardedNoteProps) {
  return (
    <ArtifactNote dim>
      Discarded <Time iso={at} as="relative" /> by {by}
      {reason ? (
        <>
          : <Inline text={reason} />
          {/[.!?]$/.test(reason.trim()) ? "" : "."}
        </>
      ) : (
        "."
      )}{" "}
      Removed from list and unable to create
      new versions.{" "}
      {erased ? "Files have been deleted." : `Folder is kept in .ws/artifacts/.discarded until ${until}.`}
    </ArtifactNote>
  );
}
