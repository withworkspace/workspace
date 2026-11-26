import css from "./Spacer.module.css";

// Spacer takes the room left in a row, so what's after it is at the end.
export function Spacer() {
  return <span className={css.spacer} />;
}
