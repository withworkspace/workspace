import type { Bundle } from "../../api";
import { type BundleGroup, groupTitles, standing } from "../verifyRows";
import { MissionVerifyMenu } from "./MissionVerifyMenu";
import { MissionVerifyStanding } from "./MissionVerifyStanding";
import { standingTone, stepTitle } from "./words";

// VerifyBundleMenu is the bundles that aren't in the row, behind a
// button: by where they stand, those that no longer stand sitting back.
export function VerifyBundleMenu({ groups, label, pick, end }: { groups: { group: BundleGroup; bundles: Bundle[] }[]; label: string; pick: (id?: string) => void; end?: boolean }) {
  return (
    <MissionVerifyMenu
      label={label}
      end={end}
      onPick={pick}
      groups={groups.map((g) => ({
        key: g.group,
        title: groupTitles[g.group],
        back: g.group === "stale",
        bundles: g.bundles.map((b) => ({ id: b.id, title: stepTitle(b), standing: <MissionVerifyStanding tone={standingTone(b)}>{standing(b)}</MissionVerifyStanding> })),
      }))}
    />
  );
}
