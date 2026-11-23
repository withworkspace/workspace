import { type ComponentProps, type ReactNode, useRef } from "react";
import { RefContext } from "../refs/context";
import { cx } from "./cx";
import { FloatingCard } from "./FloatingCard";
import css from "./HoverCard.module.css";
import { useHoverDelay } from "./useHoverDelay";

// HoverCard says more about something on hover (or focus): a card drawn
// over the page, below it, or above when there's no room below, kept on
// screen. It waits a moment before showing, so passing over doesn't flash,
// and stays while the pointer's on it.
//
// What a card says is laid out with the parts below (a head, lines, what
// was said, links); what it says of a thing is that thing's own part (a
// mission's crew: MissionCrewCard).
export function HoverCard({ trigger, children, className }: { trigger: ReactNode; children: ReactNode; className?: string }) {
  const host = useRef<HTMLSpanElement>(null);
  const hover = useHoverDelay<true>();
  return (
    <span
      ref={host}
      className={cx(css.host, className)}
      onMouseEnter={() => hover.over(true)}
      onMouseLeave={hover.out}
      onFocus={() => hover.over(true)}
      onBlur={hover.out}
      tabIndex={0}
    >
      {trigger}
      {hover.shown && host.current && (
        <FloatingCard anchor={host.current} onEnter={hover.stay} onLeave={hover.out}>
          {/* Inside a card, refs are words: no cards in cards. */}
          <RefContext.Provider value={undefined}>{children}</RefContext.Provider>
        </FloatingCard>
      )}
    </span>
  );
}

// HoverCardHead is a card's first line: what it's about (its mark, its
// name), and where it stands. top: its mark keeps to its name's first
// line, where the name runs to more than one.
export function HoverCardHead({ top, className, ...rest }: ComponentProps<"span"> & { top?: boolean }) {
  return <span className={cx(css.head, top && css.top, className)} {...rest} />;
}

// HoverCardTitle is its name, cut short at the card's width; wrap, said
// whole, on as many lines as it takes (where there's room: a pane).
export function HoverCardTitle({ wrap, className, ...rest }: ComponentProps<"span"> & { wrap?: boolean }) {
  return <span className={cx(css.title, wrap && css.wrap, className)} {...rest} />;
}

// HoverCardStrong is a word or two that matter, in a line.
export function HoverCardStrong({ className, ...rest }: ComponentProps<"span">) {
  return <span className={cx(css.strong, className)} {...rest} />;
}

// HoverCardLine is one thing said of it, on a line of its own; dim, one
// that matters less; wrap, one whose long words break where they must.
export function HoverCardLine({ dim, wrap, className, ...rest }: ComponentProps<"span"> & { dim?: boolean; wrap?: boolean }) {
  return <span className={cx(css.line, dim && css.dim, wrap && css.breaks, className)} {...rest} />;
}

// HoverCardSaid is what was last said of it, under a rule.
export function HoverCardSaid({ className, ...rest }: ComponentProps<"span">) {
  return <span className={cx(css.said, className)} {...rest} />;
}

// HoverCardLinks is a row of the places a card goes to.
export function HoverCardLinks({ className, ...rest }: ComponentProps<"span">) {
  return <span className={cx(css.links, className)} {...rest} />;
}

// hoverCardLinkClass is the look of a link in a card: where it goes, in
// the accent.
export function hoverCardLinkClass(className?: string): string {
  return cx(css.link, className);
}
