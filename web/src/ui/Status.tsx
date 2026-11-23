import css from "./Status.module.css";
import { cx } from "./cx";
import { Icon, type IconName } from "./Icon";
import { Spinner } from "./Spinner";
import { type Tone, toneClass } from "./tone";

// StatusInfo is where something stands, to draw: its icon, its words, and
// its tone, which is what it asks of you, not a hue.
//
//   attention  it needs you
//   progress   an agent's on it
//   waiting    waiting on something outside (checks, a reviewer)
//   done       finished, or ready to be
//   danger     failed, or stopped to ask a permission
//   quiet      nothing's happening
export interface StatusInfo {
  icon: IconName;
  tone: Tone;
  words: string;
  live?: boolean; // something's happening now: the icon pulses
}

// Status is one look for where something stands, used wherever a status
// shows: an icon and its words in its tone, or the icon alone (the words
// are then its tooltip). What stands where is worked out in statusOf.ts.
export function Status({ s, iconOnly, title, size = 14, className }: { s: StatusInfo; iconOnly?: boolean; title?: string; size?: number; className?: string }) {
  return (
    <span className={statusClass(s.tone, className)} title={title ?? (iconOnly ? s.words : undefined)}>
      {s.icon === "spinner" && s.live ? <Spinner size={size} /> : <Icon name={s.icon} size={size} className={s.live ? css.pulse : undefined} />}
      {iconOnly ? <span className="sr-only">{s.words}</span> : <span>{s.words}</span>}
    </span>
  );
}

// statusClass is a status's look for something drawn by hand: an icon
// alone in a tone, which stands for a kind of thing and says nothing.
export function statusClass(tone: Tone, className?: string): string {
  return cx(css.status, toneClass(tone), className);
}
