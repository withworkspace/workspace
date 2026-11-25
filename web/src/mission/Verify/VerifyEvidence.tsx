import type { BundleEvidence } from "../../api";
import { Inline } from "../../ui/Inline";
import { MissionVerifyEvidence } from "./MissionVerifyEvidence";
import { VerifyShot } from "./VerifyShot";

// VerifyEvidence is a file that shows a check, as it is: an image (beside
// the same on ground, where the bundle has it), a video to play, or text
// as a transcript; then what it shows, and the command that made it.
export function VerifyEvidence({ e }: { e: BundleEvidence }) {
  const pair = e.kind === "image" && !!e.before;
  return (
    <MissionVerifyEvidence pair={pair} caption={e.caption && <Inline text={e.caption} />} madeBy={e.madeBy}>
      {pair ? (
        <>
          <VerifyShot e={e.before!} label="Before, on ground" />
          <VerifyShot e={e} label="After, in the capsule" />
        </>
      ) : (
        <VerifyShot e={e} />
      )}
    </MissionVerifyEvidence>
  );
}
