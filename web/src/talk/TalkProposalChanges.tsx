import type { ReactNode } from "react";
import css from "./TalkProposalChanges.module.css";

// TalkProposalChanges is what a revision changed of the proposal it
// revises (proposal n), folded: its children, the change (given
// talkProposalChangedClass) or that its words are as they were.
export function TalkProposalChanges({ n, children }: { n: number; children: ReactNode }) {
  return (
    <details className={css.changes}>
      <summary className="dim">What changed since proposal {n}</summary>
      {children}
    </details>
  );
}

export const talkProposalChangedClass = css.changed;
