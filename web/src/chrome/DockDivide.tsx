import css from "./DockDivide.module.css";

// DockDivide is the line between two lots of a dock's tabs: those that
// are here because of the page, and those that are yours wherever you go.
export function DockDivide() {
  return <span role="separator" aria-orientation="vertical" className={css.divide} />;
}
