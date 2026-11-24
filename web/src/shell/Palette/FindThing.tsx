import type { ReactNode } from "react";
import css from "./FindThing.module.css";

// FindThing is a plugin's thing in Find's pane: what it is (children: its
// card, for a pane), laid out as the pane's own lines are. A name or a
// link in it is somewhere to go: pressing one says so (onGo), for Find
// to close.
export function FindThing({ onGo, children }: { onGo?: () => void; children: ReactNode }) {
  return (
    <div className={css.thing} onClickCapture={(e) => (e.target as HTMLElement).closest(".named-ref, a[href]") && onGo?.()}>
      {children}
    </div>
  );
}
