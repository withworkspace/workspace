import { Button } from "../ui/Button";
import { Time } from "../ui/Time";
import css from "./GuideFoot.module.css";
import { PassShow } from "./PassShow";
import { textClass } from "../ui/Text";

// GuideFoot is under a guide: whether the files are read in its order,
// when it was written, and writing it again.
export function GuideFoot({ guided, at, running, onGuided, onRun }: { guided: boolean; at?: string; running: boolean; onGuided: (on: boolean) => void; onRun: () => void }) {
  return (
    <div className={css.foot}>
      <PassShow checked={guided} onChange={onGuided} title="Files and diff in the guide's order. Nothing is hidden.">
        Guide order
      </PassShow>
      <span className={css.spacer} />
      {at && <Time iso={at} as="relative" className={textClass("dim")} />}
      <Button variant="ghost" size="small" onClick={onRun} disabled={running}>
        Rewrite
      </Button>
    </div>
  );
}
