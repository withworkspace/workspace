import type { ReactNode } from "react";
import type { IconName } from "../ui/Icon";
import { ReviewViewRow } from "./ReviewViewRow";
import { ReviewViews } from "./ReviewViews";

// ViewEntry is one of a review's views in the list down the side: what's
// read beside the changes (the overview, the guide, what's been said, the
// passes), each shown alone in the pane.
export interface ViewEntry {
  key: string;
  label: string;
  icon: IconName;
  n: number; // its key: 1 for the first
  mark?: ReactNode; // how it stands (each pass), beside its name
  count?: number; // how much is in it (what's been said)
  news?: string; // what's new in it since you looked
}

// ViewList is the review's views, above the changes.
export function ViewList({ views, here, onPick }: { views: ViewEntry[]; here: string; onPick: (key: string) => void }) {
  if (!views.length) return null;
  return (
    <ReviewViews>
      {views.map((v) => (
        <ReviewViewRow key={v.key} icon={v.icon} label={v.label} n={v.n} here={here === v.key} stands={v.mark} count={v.count} news={v.news} onPick={() => onPick(v.key)} />
      ))}
    </ReviewViews>
  );
}
