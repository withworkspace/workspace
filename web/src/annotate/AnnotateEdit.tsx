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
        <Button variant="link" className={css.seeApplied} onClick={onPreview} title="See it in the document, as a tracked change (v)">
          <Icon name="eye" size={12} /> See it in place
        </Button>
      }
      actions={
        settling ? (
          <AnnotateSuggestionBusy>Writing it in…</AnnotateSuggestionBusy>
        ) : (
          <>
            <Button variant="ghost" size="small" disabled={settling} onClick={() => onSettle("decline")} title="Decline it (d)">
              Decline {active && <Keys k="d" />}
            </Button>
            <Button variant="primary" size="small" disabled={settling} onClick={() => onSettle("accept")} title={landing ? `Accept it, and land it on ${landing} (a)` : "Accept it (a)"}>
              {landing ? `Land it on ${landing}` : "Accept"} {active && <Keys k="a" />}
            </Button>
          </>
        )
      }
    >
      <Change bare wrap was={[from]} now={to === "" ? [] : [to]} gone="(delete it)" />
      {landing && (
        <AnnotateSettled>
          Accepting writes it in and lands it on {landing} at once: there's no pull request. <Keys k="a" /> again does it.
        </AnnotateSettled>
      )}
    </AnnotateSuggestion>
  );
}
