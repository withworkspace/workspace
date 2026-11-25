import type { ReactNode } from "react";
import { Link } from "react-router";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import type { Tone } from "../ui/tone";
import css from "./TeamKnowLine.module.css";
import { textClass } from "../ui/Text";

type TeamKnowLineProps = {
  // Its kind, in a chip: what it is, in words and an icon.
  kind: { label: string; icon: IconName; title: string; tone: Tone };
  // Where the line's words come from (the files near yours), on hover.
  title?: string;
  children: ReactNode;
};

// TeamKnowLine is one thing worth knowing of a change, under its row: its
// kind, then what it says. The kinds of a row's lines are one column, so
// what they say starts in line.
export function TeamKnowLine({ kind, title, children }: TeamKnowLineProps) {
  return (
    <span className={css.line} title={title}>
      <Chip tone={kind.tone} className={css.kind} title={kind.title}>
        <Icon name={kind.icon} size={11} /> {kind.label}
      </Chip>
      <span>{children}</span>
    </span>
  );
}

// TeamKnowFile is the file a thing worth knowing rests on, after what it
// says: it opens in Files (and not the row it's in).
export function TeamKnowFile({ to, title, children }: { to: string; title: string; children: ReactNode }) {
  return (
    <Link to={to} className={cx(textClass("mono"), css.file)} onClick={(e) => e.stopPropagation()} title={title}>
      {children}
    </Link>
  );
}
