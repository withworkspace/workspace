import type { ReactNode } from "react";
import css from "./FindFoot.module.css";

// FindFoot is the line under Find's list: the keys that do something
// from here, each a FindFootKey, and at its end something to know of
// what's shown (aside: how old it is). A phone has no keys: it isn't
// drawn there.
export function FindFoot({ aside, children }: { aside?: ReactNode; children: ReactNode }) {
  return (
    <div className={css.foot}>
      {children}
      {aside && <span className={css.aside}>{aside}</span>}
    </div>
  );
}
