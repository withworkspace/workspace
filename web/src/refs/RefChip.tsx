import type { ReactNode } from "react";
import { Chip } from "../ui/Chip";
import type { Tone } from "../ui/tone";
import css from "./RefChip.module.css";

// RefChip is a word or two in a chip at the end of a line of a ref's
// card (where a pull request's checks stand, a plugin's thing's badge),
// kept a little from the words before it.
export function RefChip({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <Chip tone={tone} className={css.chip}>
      {children}
    </Chip>
  );
}
