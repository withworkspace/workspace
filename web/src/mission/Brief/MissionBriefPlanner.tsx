import type { ReactNode, Ref, UIEvent } from "react";
import { Link } from "react-router";
import { Button } from "../../ui/Button";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { TurnAvatar } from "../../ui/Turn";
import css from "./MissionBriefPlanner.module.css";

type MissionBriefPlannerProps = {
  // What it's called, for whoever can't see it.
  label: string;
  // In the documents' place, while there's nothing drafted to be beside.
  centre?: boolean;
  // Its conversation's being read: its place is kept, with nothing in it,
  // so the documents don't move when it's there.
  reading?: boolean;
  // Its conversation, on its own page.
  talk?: { id: string; to: string };
  // What its head offers besides.
  action?: ReactNode;
  onFold?: () => void;
  // What's been said: it scrolls by itself on a wide window.
  body?: Ref<HTMLDivElement>;
  onScroll?: (e: UIEvent<HTMLDivElement>) => void;
  // It's said aloud as it's added to.
  live?: boolean;
  children?: ReactNode;
  // Where it's asked.
  foot?: ReactNode;
};

// MissionBriefPlanner is the planner's pane in the Brief tab: its head
// (who it is, the way to its conversation's page, and folding it away),
// what's been said, and the field it's asked in at its foot, which says
// it's that (data-brief-ask), for the tab's keys.
export function MissionBriefPlanner({ label, centre, reading, talk, action, onFold, body, onScroll, live, children, foot }: MissionBriefPlannerProps) {
  if (reading) return <aside className={cx(css.planner, centre && css.centre)} aria-label={label} aria-busy />;
  return (
    <aside className={cx(css.planner, centre && css.centre)} aria-label={label}>
      <div className={css.head}>
        <TurnAvatar by="agent" />
        <span className={css.name}>Planner</span>
        {talk && (
          <Link to={talk.to} className={css.id} title="The whole conversation, on its own page">
            <Icon name="message" size={11} /> {talk.id}
          </Link>
        )}
        <span className={css.spacer} />
        {action}
        {onFold && (
          <Button variant="ghost" square type="button" onClick={onFold} title="Fold the planner away (p)" aria-label="Fold the planner away">
            <Icon name="panel" size={14} />
          </Button>
        )}
      </div>
      <div className={css.body} ref={body} aria-live={live ? "polite" : undefined} onScroll={onScroll}>
        {children}
      </div>
      <div className={css.foot} data-brief-ask>
        {foot}
      </div>
    </aside>
  );
}

// missionBriefCallsClass places an agent's steps (Calls) in the planner's
// conversation.
export const missionBriefCallsClass = () => css.calls;
