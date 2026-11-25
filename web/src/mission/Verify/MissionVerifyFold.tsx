import type { ReactNode } from "react";
import css from "./MissionVerifyFold.module.css";

// MissionVerifyFold is a list of the open bundle's that's folded until
// it's asked for: what it didn't verify, what its crew noticed, how to
// try it.
export function MissionVerifyFold({ title, items }: { title: ReactNode; items: ReactNode[] }) {
  return (
    <details className={css.fold}>
      <summary>
        {title}
        <span className={css.n}>{items.length}</span>
      </summary>
      <ul>
        {items.map((x, i) => (
          <li key={i}>{x}</li>
        ))}
      </ul>
    </details>
  );
}
