import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import css from "./AnnotateSheet.module.css";

// AnnotateSheet is where a thread (or a new comment) opens on a narrow
// window, where there's no margin: a sheet up from the bottom over a
// dimmed page, closed by its button or a press on the page.
export function AnnotateSheet({ label, onClose, children }: { label: string; onClose: () => void; children: ReactNode }) {
  return (
    <>
      <div className={css.scrim} onClick={onClose} />
      <div className={css.sheet} role="dialog" aria-modal="true" aria-label={label}>
        <div className={css.head}>
          <span className={css.grip} aria-hidden />
          <Button variant="ghost" size="small" square className={css.close} onClick={onClose} aria-label="Close">
            <Icon name="x" size={14} />
          </Button>
        </div>
        {children}
      </div>
    </>
  );
}
