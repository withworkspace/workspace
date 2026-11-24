import { useState } from "react";
import { useCommandLabels } from "../workspace/store";
import css from "./Doing.module.css";
import { type Call, Calls } from "./Calls";
import { cx } from "./cx";
import { stepDoing } from "./commandLabel";
import { Icon } from "./Icon";
import { Button } from "./Button";
import { TypingDots } from "./TypingDots";

// Doing is a run going now as one line: what the agent's doing this
// moment, the newest step taking the place of the one before, as Claude
// shows it. Its steps are beside it, under "n steps", for anyone who wants
// them; once the run's done its turn folds them as every turn does.
export function Doing({ calls, root, idle = "Thinking" }: { calls: Call[]; root?: string; idle?: string }) {
  const labels = useCommandLabels();
  const [open, setOpen] = useState(false);
  const n = calls.length;
  const last = calls[n - 1];
  const now = last ? stepDoing(last, labels) : idle;
  return (
    <div className={css.doing}>
      <div className={css.line} aria-live="polite">
        <TypingDots className={css.dots} />
        {/* Keyed by the step, so a new one comes in as new. */}
        <span key={n} className={css.now} title={last?.input}>
          {now}…
        </span>
        {n > 0 && (
          <Button variant="link" type="button" className={css.steps} onClick={() => setOpen((o) => !o)} aria-expanded={open}>
            {n} {n === 1 ? "step" : "steps"}
            <Icon name="chevron-right" size={11} className={cx(css.chevron, open && css.open)} />
          </Button>
        )}
      </div>
      {open && <Calls calls={calls.map((c, i) => (i === n - 1 && !c.result ? { ...c, pending: true } : c))} root={root} open live />}
    </div>
  );
}
