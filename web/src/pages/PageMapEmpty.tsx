import css from "./PageMapEmpty.module.css";

// PageMapEmpty is the map of a page with nothing to map yet.
export function PageMapEmpty() {
  return <p className={css.empty}>As you write, this is the page in outline: its headings, its cells, what's open, and the files it rests on.</p>;
}
