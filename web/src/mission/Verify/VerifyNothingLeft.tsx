import type { Bundle } from "../../api";
import { Empty } from "../../ui/Empty";
import { nothingLeftWords, queued } from "../verifyRows";
import { MissionVerify } from "./MissionVerify";
import { VerifyBundleMenu } from "./VerifyBundleMenu";

// VerifyNothingLeft is the whole tab when no bundle is yours to approve
// and you haven't opened one: that, where the rest stand, and a way to
// them.
export function VerifyNothingLeft({ bundles, pick }: { bundles: Bundle[]; pick: (id?: string) => void }) {
  return (
    <MissionVerify done fills>
      <Empty
        big
        done
        icon="check"
        title="Nothing left to approve"
        action={<VerifyBundleMenu groups={queued(bundles)} label={bundles.length === 1 ? "View bundle" : `View ${bundles.length} bundles`} pick={pick} />}
      >
        {nothingLeftWords(bundles)} Bundles that need you show here and on the tab.
      </Empty>
    </MissionVerify>
  );
}
