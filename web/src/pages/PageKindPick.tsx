import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./PageKindPick.module.css";

// PageKindPick is a kind of document, picked (on) or not.
export function PageKindPick({ on, title, onClick, children }: { on: boolean; title?: string; onClick: () => void; children: ReactNode }) {
  return (
    <button role="radio" aria-checked={on} className={cx(css.pick, on && css.on)} onClick={onClick} title={title}>
      {children}
    </button>
  );
}
