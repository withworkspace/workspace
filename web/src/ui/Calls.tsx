import { useState } from "react";
import { useCommandLabels } from "../workspace/store";
import css from "./Calls.module.css";
import { Button } from "./Button";
import { type Call, CallLine } from "./CallLine";
import { Chip } from "./Chip";
import { cx } from "./cx";
import { Icon } from "./Icon";

export type { Call } from "./CallLine";

// Calls are the steps an agent took on the way to a reply, each a line
// (CallLine.tsx). Folded under "n steps", but for a run going now. The
// same in conversations, a crew's transcript and an orchestrator's run.
//
// live is whether the agent's running (a pending step spins); root
// shortens paths; fileLink is where a file it edited can be read; and
// className is the page's own, to place them.
//
// tail: opened, only the last few steps show, the ones before them under
// a line that says how many (and shows them, asked): a run going now
// stays a few lines tall however long it goes on, so the page under it
// doesn't grow step by step and fall back when it's done.
export function Calls({
  calls,
  open,
  tail,
  root,
  live,
  fileLink,
  className,
}: {
  calls: Call[];
  open?: boolean;
  tail?: number;
  root?: string;
  live?: boolean;
  fileLink?: (path: string) => string | undefined;
  className?: string;
}) {
  const [all, setAll] = useState(false);
  const labels = useCommandLabels();
  if (!calls.length) return null;
  const refused = calls.filter((c) => c.refused).length;
  const failed = calls.filter((c) => c.error && !c.refused).length;
  const edits = calls.filter((c) => c.edit).length;
  // (Two more than tail before any are held back: a line saying "1
  // earlier step" would save nothing.)
  const from = open && tail && !all && calls.length > tail + 1 ? calls.length - tail : 0;
  const list = (
    <ol className={css.list}>
      {calls.map((c, i) => i >= from && <CallLine key={i} c={c} root={root} fileLink={fileLink} labels={labels} running={live && (c.pending ?? false)} />)}
    </ol>
  );
  if (open)
    return (
      <div className={className}>
        {from > 0 && (
          <Button variant="link" type="button" onClick={() => setAll(true)}>
            <Icon name="chevron-right" size={12} /> {from} earlier steps
          </Button>
        )}
        {list}
      </div>
    );
  return (
    <details className={cx(css.fold, className)}>
      <summary className={css.summary}>
        <Icon name="chevron-right" size={12} className={css.chevron} />
        {calls.length} {calls.length === 1 ? "step" : "steps"}
        {edits > 0 && <Chip tone="quiet">{edits === 1 ? "1 edit" : `${edits} edits`}</Chip>}
        {refused > 0 && <Chip tone="danger">{refused} refused</Chip>}
        {failed > 0 && <Chip tone="quiet">{failed} failed</Chip>}
      </summary>
      {list}
    </details>
  );
}
