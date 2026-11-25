import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import css from "./TalkProposalStale.module.css";

// TalkProposalStale says what's moved on since an artifact's version was
// proposed (versions added since), in the attention colour.
export function TalkProposalStale({ children }: { children: ReactNode }) {
  return (
    <div className={css.stale}>
      <Icon name="alert" size={13} /> {children}
    </div>
  );
}
