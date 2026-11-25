import type { DocHit } from "../api";
import { RowLine } from "../ui/Rows";
import css from "./SpaceHitLine.module.css";

// SpaceHitLine is where a search of a space found a document: the
// section, and its words with those found marked.
export function SpaceHitLine({ heading, snippet }: { heading?: string; snippet: DocHit["snippet"] }) {
  return (
    <RowLine className={css.line}>
      {heading && <span className={css.heading}>{heading} ›</span>}
      <span className={css.snippet}>{snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}</span>
    </RowLine>
  );
}
