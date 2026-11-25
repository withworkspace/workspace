import css from "./LiveDot.module.css";

// LiveDot says something's going on now: a dot in its place's colour,
// pulsing. It's in a chip beside who's working, and it's the mark of a
// run that's up.
export function LiveDot() {
  return <span className={css.dot} />;
}
