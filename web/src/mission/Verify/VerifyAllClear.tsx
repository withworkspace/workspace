import type { Bundle } from "../../api";
import { allClearWords, type CheckRow } from "../verifyRows";
import { MissionVerifyClear } from "./MissionVerifyClear";
import { settles } from "./words";

// VerifyAllClear is the check's pane once the last check left for you is
// approved: that, what still stands as it was, and the bundle's approval
// at a size you can't miss.
export function VerifyAllClear({ b, rows, busy, settle, again }: { b: Bundle; rows: CheckRow[]; busy: boolean; settle: () => void; again: () => void }) {
  return (
    <MissionVerifyClear id={b.id} after={settles(b)} busy={busy} onApprove={settle} onAgain={again}>
      {allClearWords(b, rows)}
    </MissionVerifyClear>
  );
}
