import type { ReactNode } from "react";
import { Dialog } from "../ui/Dialog";
import css from "./PullRequestSubmit.module.css";

// PullRequestSubmit is the dialog a pull request's review is posted to
// GitHub from: what it means to (intro), what's done to every layer at
// once (all), each layer's review (layers, which scroll), then what's
// pressed: start (discarding the draft) and end (cancel, post).
export function PullRequestSubmit({ title, intro, all, start, end, onClose, children }: { title: string; intro: ReactNode; all?: ReactNode; start?: ReactNode; end: ReactNode; onClose: () => void; children: ReactNode }) {
  return (
    <Dialog title={title} onClose={onClose} className={css.dialog} wide>
      <p>{intro}</p>
      {all && <div className={css.all}>{all}</div>}
      <div className={css.layers}>{children}</div>
      <div className={css.actions}>
        {start}
        <span className={css.spacer} />
        {end}
      </div>
    </Dialog>
  );
}
