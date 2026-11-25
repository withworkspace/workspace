import type { ThreadKind } from "../api";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { kindIcon } from "./words";
import css from "./AnnotateThreadMarker.module.css";

const kinds: Record<ThreadKind, string> = { ask: css.ask, change: css.change, note: css.note };

// AnnotateThreadMarker stands in for a thread's card on a narrow window,
// where there's no margin: a mark in its kind's colour beside its text
// (top), with how many have said something in it. Pressed, the thread
// opens in a sheet. yours: it's waiting on you; on: it's in focus.
export function AnnotateThreadMarker({
  kind,
  top,
  turns,
  resolved,
  yours,
  on,
  label,
  onPress,
}: {
  kind: ThreadKind;
  top: number;
  turns: number;
  resolved?: boolean;
  yours?: boolean;
  on?: boolean;
  label: string;
  onPress: () => void;
}) {
  return (
    <button
      className={cx(css.marker, kinds[kind], resolved && css.resolved, yours && css.yours, on && css.on)}
      style={{ top }}
      onClick={(e) => {
        e.stopPropagation();
        onPress();
      }}
      aria-label={label}
    >
      <Icon name={resolved ? "check" : kindIcon[kind]} size={11} />
      {turns > 1 && <span>{turns}</span>}
    </button>
  );
}
