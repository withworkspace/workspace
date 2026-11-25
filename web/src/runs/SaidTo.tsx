import { Link } from "react-router";
import type { Said } from "./runRows";

// SaidTo is something said of a run (where it is, who it's for), going
// where it says when it says (to). span: its words in a span of their
// own, where they'd otherwise run on in a flex column.
export function SaidTo({ s, span }: { s: Said; span?: boolean }) {
  if (s.to) return <Link to={s.to}>{s.text}</Link>;
  return span ? <span>{s.text}</span> : <>{s.text}</>;
}
