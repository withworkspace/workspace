import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Time } from "../../ui/Time";
import css from "./MissionWorkRow.module.css";

type MissionWorkRowProps = {
  id?: string;
  // What it is, at its head: an icon, or the badge of the crew who left it.
  icon: ReactNode;
  name: ReactNode;
  // A capsule's own name, where that isn't the mission's.
  slug?: ReactNode;
  // How much has changed in it.
  stat?: ReactNode;
  // At its end: who's there now, where it stands.
  end?: ReactNode;
  // When it was made, where it has nothing to say of how it stands.
  when?: string;
  // It's the one that's open.
  selected?: boolean;
  // One that doesn't need you now (up next, done): its name is quieter.
  quiet?: boolean;
  // An output: there's room at its head for a crew's badge.
  output?: boolean;
  // Its name is a sentence (a handoff's words), not a title.
  plain?: boolean;
  title?: string;
  onSelect: () => void;
  // A double click: opened where it's read, or reviewed.
  onOpen?: () => void;
};

// MissionWorkRow is one thing a mission has made, in a line: a capsule or
// an output. A click (or ⏎) opens it beside the list.
export function MissionWorkRow({ id, icon, name, slug, stat, end, when, selected, quiet, output, plain, title, onSelect, onOpen }: MissionWorkRowProps) {
  return (
    <div
      id={id}
      className={cx(css.row, selected && css.selected, quiet && css.quiet, output && css.output, plain && css.plain)}
      role="button"
      tabIndex={0}
      aria-current={selected}
      title={title}
      onClick={onSelect}
      onDoubleClick={onOpen}
      onKeyDown={(e) => e.key === "Enter" && (e.stopPropagation(), onSelect())}
    >
      {icon}
      <span className={css.title}>
        <span className={css.name}>{name}</span>
        {slug && <span className={cx("mono", css.slug)}>{slug}</span>}
        {stat && <span className={css.stat}>{stat}</span>}
      </span>
      <span className={css.end}>
        {end}
        {when && <Time iso={when} as="relative" className={css.when} />}
      </span>
    </div>
  );
}
