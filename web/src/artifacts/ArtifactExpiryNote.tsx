import { Icon } from "../ui/Icon";
import { ArtifactNote } from "./ArtifactNote";

// ArtifactExpiryNote says a live artifact is in its last week (words:
// "expires in 4 days"), or has expired.
export function ArtifactExpiryNote({ words, expired }: { words: string; expired: boolean }) {
  return (
    <ArtifactNote expiry={expired ? "expired" : "soon"}>
      <Icon name="clock" size={12} />{" "}
      {expired ? "Expired: it's had no new version for a while, and the daemon discards it today. Promote it to keep it." : `It ${words}: a new version, or promoting it, keeps it.`}
    </ArtifactNote>
  );
}
