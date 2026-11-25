import type { ReactNode } from "react";
import css from "./MissionVerifyYourself.module.css";

// MissionVerifyYourself is how to see a check for yourself: something to
// do, or to look for, a line each.
export function MissionVerifyYourself({ items }: { items: ReactNode[] }) {
  return (
    <div className={css.yourself}>
      <h4>To see it yourself</h4>
      <ul>
        {items.map((x, i) => (
          <li key={i}>{x}</li>
        ))}
      </ul>
    </div>
  );
}
