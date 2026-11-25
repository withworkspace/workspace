import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PagePanelFoot.module.css";

// PagePanelFoot is the foot of a cell's panel: what's done with the cell
// (children) and its name (id); or, under the box that asks for a
// change, what asking does (hint).
export function PagePanelFoot({ id, hint, children }: { id?: string; hint?: ReactNode; children?: ReactNode }) {
  return (
    <div className={css.foot}>
      {children}
      {id && (
        <>
          <span className="spacer" />
          <span className="dim">{id}</span>
        </>
      )}
      {hint && <span className={cx("dim", css.hint)}>{hint}</span>}
    </div>
  );
}
