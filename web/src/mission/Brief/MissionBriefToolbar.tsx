import type { ReactNode, Ref } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionBriefToolbar.module.css";

type MissionBriefToolbarProps = {
  ref?: Ref<HTMLDivElement>;
  // Where the editor puts its toolbar.
  slot: Ref<HTMLDivElement>;
  // How far in it starts: where the documents' text does.
  inset: number;
  // Which document it's for, while the other's changes are being read and
  // it isn't the one in front of you; and why that's said.
  of?: ReactNode;
  ofTitle?: string;
  // What's said where there's no toolbar (nothing's being edited): words,
  // or none of them, which leaves its place empty.
  none?: string;
};

// MissionBriefToolbar is the editor's toolbar's place in the tab's bar,
// for whichever document is being edited: under the rest of the bar, in
// it on a wide window.
export function MissionBriefToolbar({ ref, slot, inset, of, ofTitle, none }: MissionBriefToolbarProps) {
  return (
    <div className={css.toolbar} ref={ref} style={{ paddingLeft: inset }}>
      {of && (
        <span className={css.of} title={ofTitle}>
          {of}
        </span>
      )}
      <div className={css.slot} ref={slot} />
      {none !== undefined && <span className={cx(css.none, "dim")}>{none}</span>}
    </div>
  );
}
