import type { ReactNode } from "react";
import css from "./PagePassHead.module.css";

// PagePassHead is what a pass is (about), how its agent reads (how), why
// it can't be run if it can't (unavailable), and Run (action).
export function PagePassHead({ about, how, unavailable, action }: { about: ReactNode; how: ReactNode; unavailable?: ReactNode; action: ReactNode }) {
  return (
    <header className={css.head}>
      <div className={css.about}>
        <p>{about}</p>
        <p className="dim">{how}</p>
        {unavailable && <p className={css.unavailable}>It can't be run: {unavailable}.</p>}
      </div>
      {action}
    </header>
  );
}
