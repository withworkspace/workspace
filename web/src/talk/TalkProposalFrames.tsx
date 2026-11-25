import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./TalkProposalFrames.module.css";

// TalkProposalFrames holds an artifact proposed, running in its
// TalkProposalFrame; two, beside the version it revises.
export function TalkProposalFrames({ two, children }: { two?: boolean; children: ReactNode }) {
  return <div className={cx(css.frames, two && css.two)}>{children}</div>;
}
