import { Button } from "../ui/Button";
import { Time } from "../ui/Time";
import css from "./GuideFoot.module.css";
import { PassShow } from "./PassShow";

// GuideFoot is under a guide: whether the files are read in its order,
// when it was written, and writing it again.
export function GuideFoot({ guided, at, running, onGuided, onRun }: { guided: boolean; at?: string; running: boolean; onGuided: (on: boolean) => void; onRun: () => void }) {
  return (
    <div className={css.foot}>
      <PassShow checked={guided} onChange={onGuided} title="The list of files, and the diff under it, follow the guide's order. Nothing's hidden: a file it doesn't place is still there">
        Read the files in this order
      </PassShow>
      <span className={css.spacer} />
      {at && <Time iso={at} as="relative" className="dim" />}
      <Button variant="ghost" size="small" onClick={onRun} disabled={running}>
        Write it again
      </Button>
    </div>
  );
}
