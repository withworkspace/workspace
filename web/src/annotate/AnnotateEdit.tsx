import { annotateKeys } from "./keys";
import { Button } from "../ui/Button";
import { Change } from "../ui/Change";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { AnnotateSettled } from "./AnnotateSettled";
import { AnnotateSuggestion, AnnotateSuggestionBusy } from "./AnnotateSuggestion";
import css from "./AnnotateEdit.module.css";

// AnnotateEdit is a suggested edit in a thread: who suggested it, what the
// markdown says (from) and would say (to; "" takes it out), and accepting
// (written in, and committed in a capsule) or declining it; active, its
// keys are shown. landing names the branch accepting lands it on at once,
// asked before it's done; settling, it's being written in.
export function AnnotateEdit({
  by,
  from,
  to,
  active,
  settling,
  landing,
  onSettle,
  onPreview,
}: {
  by: string;
  from: string;
  to: string;
  active?: boolean;
  settling?: boolean;
  landing?: string;
  onSettle: (action: "accept" | "decline") => void;
  onPreview: () => void;
}) {
  return (
    <AnnotateSuggestion
      kind="edit"
      icon="pencil"
      title="Suggested edit"
      by={`by ${by === "agent" ? "the agent" : by}`}
      actionsStart={
        <Button variant="link" className={css.seeApplied} onClick={onPreview} title="Show as tracked change" keys={annotateKeys.tracked}>
          <Icon name="eye" size={12} /> Show in place
        </Button>
      }
      actions={
        settling ? (
          <AnnotateSuggestionBusy>Applying…</AnnotateSuggestionBusy>
        ) : (
          <>
            <Button variant="ghost" size="small" disabled={settling} onClick={() => onSettle("decline")} title="Decline" keys={annotateKeys.decline}>
              Decline {active && <Keys k={annotateKeys.decline} />}
            </Button>
            <Button variant="primary" size="small" disabled={settling} onClick={() => onSettle("accept")} title={landing ? `Accept and land on ${landing}` : "Accept"} keys={annotateKeys.accept}>
              {landing ? `Land on ${landing}` : "Accept"} {active && <Keys k={annotateKeys.accept} />}
            </Button>
          </>
        )
      }
    >
      <Change bare wrap was={[from]} now={to === "" ? [] : [to]} gone="(delete)" />
      {landing && (
        <AnnotateSettled>
          Accepting lands on {landing} immediately, without a pull request. Press <Keys k={annotateKeys.accept} /> again to
          confirm.
        </AnnotateSettled>
      )}
    </AnnotateSuggestion>
  );
}
