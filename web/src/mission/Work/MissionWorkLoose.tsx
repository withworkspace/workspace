import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { toneClass } from "../../ui/tone";
import css from "./MissionWorkLoose.module.css";

// MissionWorkLoose says, in a capsule's head, what of it isn't committed:
// in the attention tone when something isn't (loose), since reviews and
// the crew after don't see it.
export function MissionWorkLoose({ loose, children }: { loose: boolean; children: ReactNode }) {
  return (
    <span className={loose ? cx(toneClass("attention"), css.loose) : undefined} title={loose ? "Changed in the capsule and not committed: reviews and crew after don't see them" : undefined}>
      {children}
    </span>
  );
}
