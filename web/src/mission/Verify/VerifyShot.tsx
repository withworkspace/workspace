import type { BundleEvidence } from "../../api";
import { MissionVerifyShot } from "./MissionVerifyShot";
import { VerifyTranscript } from "./VerifyTranscript";

// VerifyShot is one file of a bundle's, shown as its kind: text is read
// from where it's served.
export function VerifyShot({ e, label }: { e: BundleEvidence; label?: string }) {
  return (
    <MissionVerifyShot kind={e.kind === "video" ? "video" : e.kind === "image" ? "image" : "text"} url={e.url} what={e.caption || e.name} label={label}>
      <VerifyTranscript url={e.url} />
    </MissionVerifyShot>
  );
}
