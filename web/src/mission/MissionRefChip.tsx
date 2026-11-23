import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./MissionRefChip.module.css";

// MissionRefChip is a name a mission's page knows, in words that mention
// it: a short one (a decision, a conversation) as a chip, a longer one (a
// capsule, a document's path) as its words, underlined in dots; mono, a
// path or a branch. It's what's pointed at for the name's card.
export function MissionRefChip({ words, mono, children }: { words?: boolean; mono?: boolean; children: ReactNode }) {
  return <span className={cx(words ? css.words : css.chip, mono && "mono")}>{children}</span>;
}
