import type { ReactNode } from "react";
import type { TalkProposal as Proposal } from "../api";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./TalkProposal.module.css";
import { textClass } from "../ui/Text";

// TalkProposal is a card under the reply it came with: its head (the
// kind's icon and label, its title, the proposal it revises, Edit when
// onEdit's given), what changed since the one it revises (changes), what's
// proposed (children), and its foot: what you can do (a
// TalkProposalChoice) while it's open, or what came of it (a
// TalkProposalSettled). anchor is its id on the page.
export function TalkProposal({
  state,
  anchor,
  icon,
  label,
  title,
  revises,
  onEdit,
  changes,
  foot,
  children,
}: {
  state: Proposal["state"];
  anchor?: string;
  icon: IconName;
  label: string;
  title?: string;
  revises?: number;
  onEdit?: () => void;
  changes?: ReactNode;
  foot: ReactNode;
  children: ReactNode;
}) {
  const faded = state === "declined" || state === "superseded" || state === "withdrawn";
  return (
    <div className={cx(css.card, state === "open" && css.open, faded && css.faded)} id={anchor}>
      <div className={css.head}>
        <Icon name={icon} size={14} className={css.icon} />
        <span className={css.label}>{label}</span>
        {title && <span className={css.title}>{title}</span>}
        <span className={css.spacer} />
        {!!revises && (
          <a className={cx(textClass("dim"), css.revises)} href={`#proposal-${revises}`} title={`Replaces proposal ${revises}`}>
            Revises {revises}
          </a>
        )}
        {onEdit && (
          <Button variant="ghost" size="small" onClick={onEdit}>
            <Icon name="pencil" size={12} /> Edit
          </Button>
        )}
      </div>
      {changes}
      <div className={css.body}>{children}</div>
      <div className={cx(css.foot, state !== "open" && css.settled)}>{foot}</div>
    </div>
  );
}
