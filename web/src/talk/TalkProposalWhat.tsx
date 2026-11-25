import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./TalkProposalWhat.module.css";

// TalkProposalWhat is what's proposed, in a line or a few: a decision, the
// crew's task, a comment's words; said keeps the line breaks it was
// written with. why is why, under it, quieter.
export function TalkProposalWhat({ said, why, children }: { said?: boolean; why?: boolean; children: ReactNode }) {
  return <div className={why ? css.why : cx(css.what, said && css.said)}>{children}</div>;
}
