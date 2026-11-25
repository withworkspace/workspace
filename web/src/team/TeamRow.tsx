import type { ReactNode } from "react";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import css from "./TeamRow.module.css";

type TeamRowProps = {
  id: string;
  // Where it stands now, as an icon at its head.
  status: StatusInfo;
  name: ReactNode;
  // What's beside its name: where it was made, what it changes, where it
  // stands in words.
  beside?: ReactNode;
  // What's under its name: what it does, what's worth knowing of it.
  under?: ReactNode;
  // What happened to it that day, in a line.
  words: ReactNode;
  // Who's on it.
  people: string;
  // How much it changes, where that's known.
  additions?: number;
  deletions?: number;
  at: string;
  // dated: it's not under its day, so when is said with the day.
  dated?: boolean;
  selected?: boolean;
  open?: boolean;
  // fresh: something happened to it since you last looked.
  fresh?: boolean;
  // What's shown under it while it's open (TeamEvents).
  children?: ReactNode;
  onHover: () => void;
  onToggle: () => void;
};

// TeamRow is one change on the Team page, a pull request or a run of one
// person's commits to one repo in a day: where it stands, its name and
// what's beside it, what it does, who's on it, how much it changes and
// when. A click (or ⏎) opens it on what happened to it.
export function TeamRow({ id, status, name, beside, under, words, people, additions, deletions, at, dated, selected, open, fresh, children, onHover, onToggle }: TeamRowProps) {
  return (
    <Row selected={selected} id={`team-row-${id}`} role="option" aria-selected={selected} onMouseMove={onHover}>
      <div className={rowLinkClass(css.link)} role="button" tabIndex={-1} aria-expanded={open} onClick={onToggle}>
        <Status s={status} iconOnly size={16} className={rowStatusClass()} />
        <RowMain className={css.main}>
          <RowLine>
            {fresh && <span className={css.new} title="Something's happened to it since you last looked" />}
            <RowTitle>{name}</RowTitle>
            {beside}
          </RowLine>
          {under}
          <RowSub className={css.sub}>{words}</RowSub>
        </RowMain>
        <RowMeta className={css.people} title={people}>
          {people}
        </RowMeta>
        {additions || deletions ? (
          <RowMeta className={css.size}>
            <span className="plus">+{additions ?? 0}</span> <span className="minus">−{deletions ?? 0}</span>
          </RowMeta>
        ) : (
          <RowMeta className={css.size} />
        )}
        <Time iso={at} as={dated ? "relative" : "clock"} className={css.time} />
      </div>
      {open && children}
    </Row>
  );
}
