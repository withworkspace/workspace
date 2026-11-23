import type { ReactNode } from "react";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import type { Tone } from "../../ui/tone";
import css from "./FindRow.module.css";

// FindRow is one thing Find found, to pick: its mark (an icon, a status),
// what it's called (title, a FindTitle), then more about it, quieter
// (hint), a word or two on its state (notes, in tone), and at its end
// what kind of thing it is (kind), where its group doesn't say. on: the
// cursor's on it. The pointer moving over it puts the cursor there
// (onOver), and a press picks it, in a new tab with ⌘ or Ctrl held
// (onPick).
export function FindRow({
  id,
  on,
  mark,
  title,
  hint,
  notes,
  tone,
  kind,
  onOver,
  onPick,
}: {
  id: string;
  on?: boolean;
  mark: ReactNode;
  title: ReactNode;
  hint?: string;
  notes?: string[];
  tone?: Tone;
  kind?: string;
  onOver?: () => void;
  onPick?: (tab: boolean) => void;
}) {
  return (
    <div id={id} role="option" aria-selected={!!on} className={cx(css.row, on && css.on)} onMouseMove={onOver} onClick={(e) => onPick?.(e.metaKey || e.ctrlKey)}>
      {mark}
      {title}
      {hint && <span className={css.hint}>{hint}</span>}
      {notes?.map((n) => (
        <Chip tone={tone} key={n}>
          {n}
        </Chip>
      ))}
      {kind && <span className={css.kind}>{kind}</span>}
    </div>
  );
}
