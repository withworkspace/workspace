import type { ReactNode } from "react";
import css from "./TalkProposalEdit.module.css";

// TalkProposalEdit is a proposal being edited before it's confirmed: its
// fields, the width of the card, one under another.
export function TalkProposalEdit({ children }: { children: ReactNode }) {
  return <div className={css.edit}>{children}</div>;
}
