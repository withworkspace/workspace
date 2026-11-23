import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { finderBox, finderOverlay } from "../../ui/Finder";
import { Modal } from "../../ui/Modal";
import css from "./FindWindow.module.css";

// FindWindow is Find's box over the dimmed page, drawn as a finder's is:
// down it, what's typed, the list and its foot (children), and beside
// them, where the window has room for one, a pane for what the cursor's
// on, which widens the box.
export function FindWindow({ pane, onClose, children }: { pane?: ReactNode; onClose: () => void; children: ReactNode }) {
  return (
    // Escape is Find's own: a step back out of what it's in the middle of,
    // before it closes.
    <Modal onClose={onClose} escape={false} overlayClassName={finderOverlay} className={cx(finderBox, css.window, !!pane && css.roomy)} aria-label="Find">
      <div className={css.main}>{children}</div>
      {pane}
    </Modal>
  );
}
