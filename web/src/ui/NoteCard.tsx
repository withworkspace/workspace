import type { ComponentProps, ReactNode } from "react";
import { cx } from "./cx";
import { Icon } from "./Icon";
import css from "./NoteCard.module.css";

type NoteCardProps = ComponentProps<"div"> & {
  // cursor: the keys are on it (or it's the one in focus).
  cursor?: boolean;
  // draft: yours, and not sent yet.
  draft?: boolean;
  // resolved: settled, so it's drawn back.
  resolved?: boolean;
  // found: something found that nobody's said yet; blocks, one that
  // holds things up.
  found?: boolean;
  blocks?: boolean;
  // dense: a card in a margin, in smaller words.
  dense?: boolean;
  // Of a dense one: the pointer's on what it's about, it's folded to a
  // line, it's waiting on you.
  hovered?: boolean;
  folded?: boolean;
  yours?: boolean;
};

// NoteCard is something said about a place in what's read: a comment on
// a diff's lines, a thread in a document's margin, what a pass found.
// It's a card with a head (NoteHead), what was said (NoteBody), what's
// been said since (NoteThread, of NoteTurns) and what can be done next
// (NoteHint, or a ReplyBox). Where it sits is the page's, by its class.
export function NoteCard({ cursor, draft, resolved, found, blocks, dense, hovered, folded, yours, className, ...rest }: NoteCardProps) {
  return (
    <div
      className={cx(
        css.card,
        draft && css.draft,
        cursor && css.cursor,
        resolved && css.resolved,
        found && css.found,
        found && blocks && css.blocks,
        dense && css.dense,
        dense && hovered && css.hovered,
        dense && folded && css.folded,
        dense && yours && !cursor && css.yours,
        className,
      )}
      data-note-card=""
      {...rest}
    />
  );
}

// NoteHead is the card's first line: what it is and where it stands,
// then, at its end, its name and what's pressed.
export function NoteHead({ children, end }: { children?: ReactNode; end?: ReactNode }) {
  return (
    <div className={css.head}>
      {children}
      <span className={css.spacer} />
      {end}
    </div>
  );
}

// NoteId is the card's name, in the head's end: its id, or a link to it
// where it lives elsewhere (which opens there, not the card).
export function NoteId({ href, title, children }: { href?: string; title?: string; children: ReactNode }) {
  if (!href) return <span className={css.id}>{children}</span>;
  return (
    <a className={cx(css.id, css.idLink)} href={href} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()} title={title}>
      {children} <Icon name="external" size={10} />
    </a>
  );
}

export type NoteStanding = "open" | "reopened" | "addressed" | "wontfix" | "resolved" | "crew";

const standing: Record<NoteStanding, string> = {
  open: css.open,
  reopened: css.reopened,
  addressed: css.addressed,
  wontfix: css.wontfix,
  resolved: css.settled,
  crew: css.crew,
};

// NoteStatus is where a comment stands, in a word: plain (a draft), or
// in the colour of how it stands.
export function NoteStatus({ status, children }: { status?: string; children: ReactNode }) {
  return <span className={cx(css.status, status && standing[status as NoteStanding])}>{children}</span>;
}

// NoteBody is what was said, as it was written; oneLine, as much of it
// as a line holds (a folded card's).
export function NoteBody({ oneLine, className, ...rest }: ComponentProps<"div"> & { oneLine?: boolean }) {
  return <div className={cx(css.body, oneLine && css.oneLine, className)} {...rest} />;
}

// NoteThread is what's been said since, under a line: a NoteTurn each.
export function NoteThread({ children }: { children: ReactNode }) {
  return <ul className={css.thread}>{children}</ul>;
}

// NoteTurn is one thing said since: who said it, what's to be known of
// it (its children: a status it left, a commit), then their words. quiet
// is one that isn't anyone's words (ws's own).
export function NoteTurn({ who, said, quiet, children }: { who: ReactNode; said?: ReactNode; quiet?: boolean; children?: ReactNode }) {
  return (
    <li className={cx(css.turn, quiet && css.quiet)}>
      <span className={css.who}>{who}</span>
      {children}
      {said != null && said !== "" && <span className={css.said}>{said}</span>}
    </li>
  );
}

// NoteHint is what can be done next, under the card's words, quietly: a
// key, or a link or two (a Button variant="link", the one meant with
// noteHintActionClass).
export function NoteHint({ children }: { children: ReactNode }) {
  return <div className={css.hint}>{children}</div>;
}

// noteHintActionClass is the look of the hint's first thing to do.
export function noteHintActionClass(className?: string): string {
  return cx(css.hintAction, className);
}
