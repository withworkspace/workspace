import css from "./PageMapEmpty.module.css";

// PageMapEmpty is the map of a page with nothing to map yet.
export function PageMapEmpty() {
  return <p className={css.empty}>Fills in as you write: headings, cells, open items and sources.</p>;
}
