import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import { ArtifactNote } from "./ArtifactNote";

type ArtifactPromotedNoteProps = {
  at: string;
  by: string;
  // The version chosen.
  version: number;
  // What it went into, drawn: a mission's ref, or a document's name.
  into?: ReactNode;
};

// ArtifactPromotedNote says a promoted artifact is kept for good: when,
// by whom, the version chosen and what it went into.
export function ArtifactPromotedNote({ at, by, version, into }: ArtifactPromotedNoteProps) {
  return (
    <ArtifactNote dim>
      <Icon name="check-circle" size={12} /> Promoted <Time iso={at} as="relative" /> by {by}: v{version}
      {into ? (
        <>
          , into{" "}
          {into}
        </>
      ) : null}. Kept
      permanently.
    </ArtifactNote>
  );
}
