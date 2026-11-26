import { Icon } from "../ui/Icon";
import { ArtifactNote } from "./ArtifactNote";

// ArtifactExpiryNote says a live artifact is in its last week (words:
// "expires in 4 days"), or has expired.
export function ArtifactExpiryNote({ words, expired }: { words: string; expired: boolean }) {
  return (
    <ArtifactNote expiry={expired ? "expired" : "soon"}>
      <Icon name="clock" size={12} />{" "}
      {expired ? "Expired: no new version for a while, and the Workspace discards it today. Promote it to keep it." : `Artifact ${words}. Promote to keep it, or add a version.`}
    </ArtifactNote>
  );
}
