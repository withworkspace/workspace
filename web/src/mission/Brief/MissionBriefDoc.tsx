import type { ReactNode } from "react";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import type { DocId } from "./docState";
import css from "./MissionBriefDoc.module.css";

type MissionBriefDocProps = {
  id: DocId;
  title: ReactNode;
  // How many changes are still to read in it, in words.
  toRead?: ReactNode;
  // What's done to it, at its head's end.
  actions?: ReactNode;
  // Beside the other document: it scrolls by itself.
  side?: boolean;
  children: ReactNode;
};

// MissionBriefDoc is the brief or the plan in the Brief tab: a small head
// (what it is, its changes to read, what's done to it) over the document.
// It says which it is (data-brief-doc) and where its head is
// (data-brief-doc-head): what the tab's code finds them by.
export function MissionBriefDoc({ id, title, toRead, actions, side, children }: MissionBriefDocProps) {
  return (
    <section className={cx(css.doc, side && css.side)} data-brief-doc={id}>
      <h2 className={css.head} data-brief-doc-head>
        {title}
        {toRead && (
          <Chip tone="attention" className={css.chip}>
            {toRead}
          </Chip>
        )}
        <span className={css.spacer} />
        {actions}
      </h2>
      {children}
    </section>
  );
}

// missionBriefDocSelector is the selector of one of the documents, for a
// rule written as the mission moves (where it is on its plan).
export const missionBriefDocSelector = (id: DocId) => `.${css.doc}[data-brief-doc="${id}"]`;
