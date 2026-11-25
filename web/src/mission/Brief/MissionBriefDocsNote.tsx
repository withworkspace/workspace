import type { ReactNode } from "react";
import { Callout } from "../../ui/Callout";
import { cx } from "../../ui/cx";
import css from "./MissionBriefDocsNote.module.css";

type MissionBriefDocsNoteProps = {
  tone: "attention" | "danger";
  title: ReactNode;
  // The documents are side by side: it's across both.
  side?: boolean;
  children: ReactNode;
};

// MissionBriefDocsNote is something wrong with the brief or the plan,
// said among them at their measure: it was proposed against an earlier
// version, the plan doesn't read, ws didn't save it.
export function MissionBriefDocsNote({ tone, title, side, children }: MissionBriefDocsNoteProps) {
  return (
    <Callout className={cx(css.note, side && css.side)} tone={tone} icon="alert" title={title}>
      {children}
    </Callout>
  );
}
