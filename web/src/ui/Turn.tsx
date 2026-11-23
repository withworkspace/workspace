import type { ComponentProps, ReactNode, Ref } from "react";
import { cx } from "./cx";
import { Icon, type IconName } from "./Icon";
import { Time } from "./Time";
import css from "./Turn.module.css";

// Whose turn it is: yours, an agent's, ws's own word, or crew's.
export type TurnBy = "you" | "agent" | "ws" | "crew";

// Yours and ws's have a look of their own; an agent's and crew's are the
// plain text.
const whose: Record<TurnBy, string | undefined> = { you: css.you, agent: undefined, ws: css.ws, crew: undefined };

type TurnProps = Omit<ComponentProps<"div">, "children"> & {
  by: TurnBy;
  // Who said it and when, over it (a TurnHead). What's said is then set
  // in under it.
  head?: ReactNode;
  // dense: beside something else (a page, a document, in a panel), where
  // there's no room for who said it at each turn, and the words are
  // smaller.
  dense?: boolean;
  // landed: it was there already (the run going until now), so it
  // doesn't come in again.
  landed?: boolean;
  // running: the agent's turn going now.
  running?: boolean;
  // What's in it, in order: its steps (Calls), its words (a TurnSaid),
  // what came with them, and, while it's running, what it's doing (Doing).
  children?: ReactNode;
};

// Turn is one turn of an exchange with an agent: something you said, what
// it answered (its steps over its words), or a word of ws's own. Yours
// keep to the right, in a bubble; the rest are text, to the left. Every
// place there's such an exchange draws it with these: a conversation's
// page, crew's transcript, the brief's planner, the conversation beside a
// page, a cell.
export function Turn({ by, head, dense, landed, running, className, children, ...rest }: TurnProps) {
  return (
    <div className={cx(css.turn, whose[by], dense && css.dense, landed && css.landed, running && css.running, className)} data-turn={by} {...rest}>
      {head}
      {head ? <div className={css.body}>{children}</div> : children}
    </div>
  );
}

const avatars: Record<Exclude<TurnBy, "crew">, IconName> = { you: "user", agent: "sparkle", ws: "info" };
const avatarOf: Record<Exclude<TurnBy, "crew">, string | undefined> = { you: css.avatarYou, agent: css.avatarAgent, ws: undefined };

// TurnAvatar is the round mark of who said it: you, an agent, ws. (Crew
// have their own: a CrewBadge.)
export function TurnAvatar({ by, className }: { by: Exclude<TurnBy, "crew">; className?: string }) {
  return (
    <span className={cx(css.avatar, avatarOf[by], className)} aria-hidden>
      <Icon name={avatars[by]} size={12} />
    </span>
  );
}

// TurnHead is who said it, and when: their mark (a TurnAvatar, or crew's
// badge), their name, the time.
export function TurnHead({ avatar, who, at }: { avatar: ReactNode; who?: ReactNode; at?: string }) {
  return (
    <div className={css.head}>
      {avatar}
      {who != null && <span className={css.who}>{who}</span>}
      {at && <Time iso={at} className={css.time} />}
    </div>
  );
}

// TurnSaid is a turn's words. An agent's are markdown, given as its
// children; yours are as you wrote them (written), in a bubble.
export function TurnSaid({ written, className, ref, ...rest }: ComponentProps<"div"> & { written?: boolean; ref?: Ref<HTMLDivElement> }) {
  return <div ref={ref} className={cx(css.said, written && css.written, className)} {...rest} />;
}

// turnTimeClass is a turn's time's look, for a line that stands in for a
// turn and says when (a card you settled).
export function turnTimeClass(className?: string): string {
  return cx(css.time, className);
}
