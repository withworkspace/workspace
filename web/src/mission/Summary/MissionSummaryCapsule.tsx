import { missionKeys } from "../keys";
import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./MissionSummaryCapsule.module.css";
import { Text, textClass } from "../../ui/Text";

// MissionSummaryCapsule is a capsule in a line on a mission's Summary: its
// name and branch (where it isn't the capsule's name), where it stands
// (its badges), and a way to review it. The Work tab has the rest. id is
// what the page's keys find it by; selected, their cursor is on it.
export function MissionSummaryCapsule({
  id,
  name,
  branch,
  badges,
  selected,
  onReview,
}: {
  id: string;
  name: ReactNode;
  branch?: string;
  badges?: ReactNode;
  selected?: boolean;
  // There to review: its folder's still there.
  onReview?: () => void;
}) {
  return (
    <div id={id} className={cx(css.card, selected && css.selected)}>
      <div className={css.line}>
        <Icon name="box" size={14} className={textClass("dim")} />
        <Text mono className={css.name}>{name}</Text>
        {branch && (
          <Chip tone="quiet" title="Branch">
            <Icon name="branch" size={11} />
            {branch}
          </Chip>
        )}
        {badges}
        <span className={css.spacer} />
        {onReview && (
          <Button size="small" onClick={onReview} title="Review changes" keys={missionKeys.review}>
            <Icon name="eye" /> Review
          </Button>
        )}
      </div>
    </div>
  );
}
