import type { ReactNode } from "react";
import css from "./PagePassMeta.module.css";

// PagePassMeta is when a pass last ran (at), and whether what it found is
// shown on the page (children: a PagePassShown).
export function PagePassMeta({ at, children }: { at?: ReactNode; children: ReactNode }) {
  return (
    <div className={css.meta}>
      {at && <span className="dim">{at}</span>}
      <span className="spacer" />
      {children}
    </div>
  );
}
