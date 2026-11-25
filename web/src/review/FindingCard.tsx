import type { FindingAct } from "./findings";
import type { Row } from "./model";
import { ReviewFinding } from "./ReviewFinding";
import { ReviewFindingActions } from "./ReviewFindingActions";

// FindingCard draws a pass's finding, under its lines, as a ReviewFinding
// with what's done with it.
export function FindingCard({
  row,
  cursor,
  dismissing,
  canAsk,
  onRow,
  onAct,
  onDismissing,
}: {
  row: Extract<Row, { type: "finding" }>;
  cursor: boolean;
  dismissing: boolean;
  canAsk: boolean;
  onRow: (key: string) => void;
  onAct: FindingAct;
  onDismissing: (key?: string) => void;
}) {
  const { finding: f, title, pass } = row.found;
  const on = { pass, finding: f };
  return (
    <ReviewFinding
      id={`row-${row.key}`}
      title={title}
      finding={f}
      cursor={cursor}
      onClick={() => onRow(row.key)}
      actions={
        <ReviewFindingActions
          dismissing={dismissing}
          cursor={cursor}
          canAsk={canAsk}
          onTake={() => onAct(on, "taken")}
          onDismissing={(v) => onDismissing(v ? row.key : undefined)}
          onDismiss={(why) => onAct(on, "dismissed", why)}
          onAsk={() => onAct(on, "ask")}
        />
      }
    />
  );
}
