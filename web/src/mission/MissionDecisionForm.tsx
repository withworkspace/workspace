import { useEffect, useRef, useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { TextArea } from "../ui/TextArea";
import css from "./MissionDecisionForm.module.css";

// What a form of a decision's is for: one of your own recorded, one
// recorded in place of another, or one withdrawn.
export type DecisionFormKind = "record" | "replace" | "withdraw";

const words: Record<DecisionFormKind, { title: (of?: string) => string; send: string; sending: string }> = {
  record: { title: () => "A decision of yours", send: "Record", sending: "Recording…" },
  replace: { title: (of) => `In place of ${of}`, send: "Replace it", sending: "Replacing…" },
  withdraw: { title: (of) => `Withdraw ${of}`, send: "Withdraw it", sending: "Withdrawing…" },
};

// MissionDecisionForm is where a decision is written: what was decided
// and why (to record one, or one in place of another, which starts from
// that one's words), or only why (to withdraw one, which has to say).
// ⌘⏎ sends it, Escape puts it away. A decision is never edited, so what's
// replaced or withdrawn stays in the record, and the form says so.
export function MissionDecisionForm({
  kind,
  of,
  what: was = "",
  busy,
  error,
  onSend,
  onCancel,
}: {
  kind: DecisionFormKind;
  // The decision replaced or withdrawn (d4).
  of?: string;
  // What's replaced, to start from.
  what?: string;
  busy?: boolean;
  error?: string;
  onSend: (d: { what: string; why: string }) => void;
  onCancel: () => void;
}) {
  const [what, setWhat] = useState(was);
  const [why, setWhy] = useState("");
  const first = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    const el = first.current;
    if (!el) return;
    el.focus({ preventScroll: true });
    el.setSelectionRange(el.value.length, el.value.length);
  }, []);
  const withdrawing = kind === "withdraw";
  // One in place of another has to say something else: the same words
  // again would be the decision it replaces.
  const same = kind === "replace" && !!what.trim() && what.trim() === was.trim();
  const ready = withdrawing ? !!why.trim() : !!what.trim() && !same;
  const send = () => ready && !busy && onSend({ what: what.trim(), why: why.trim() });
  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      send();
    } else if (e.key === "Escape") {
      // The form's own: not the page's way back.
      e.preventDefault();
      onCancel();
    }
  };
  const rows = (text: string) => Math.min(6, Math.max(2, Math.ceil(text.length / 70)));
  const w = words[kind];
  return (
    <form
      className={css.form}
      aria-label={w.title(of)}
      onSubmit={(e) => {
        e.preventDefault();
        send();
      }}
    >
      <div className={css.head}>
        <Icon name="gavel" size={12} /> {w.title(of)}
        <span className={css.note}>
          {kind === "record" && "Yours stand over what the plan asks, and crew and the orchestrator are shown them."}
          {kind === "replace" && `${of} stays in the record, as replaced.`}
          {kind === "withdraw" && `${of} stays in the record, as withdrawn.`}
        </span>
      </div>
      {!withdrawing && (
        <TextArea ref={first} className={css.field} value={what} rows={rows(what)} placeholder="The decision, in a sentence" aria-label="The decision" onChange={(e) => setWhat(e.target.value)} onKeyDown={keys} />
      )}
      <TextArea
        ref={withdrawing ? first : undefined}
        className={css.field}
        value={why}
        rows={rows(why)}
        placeholder={withdrawing ? "Why it no longer holds" : "Why"}
        aria-label={withdrawing ? "Why it's withdrawn" : "Why"}
        onChange={(e) => setWhy(e.target.value)}
        onKeyDown={keys}
      />
      {error && (
        <div className={css.error} role="alert">
          {error}
        </div>
      )}
      <div className={css.foot}>
        <Button variant="ghost" size="small" onClick={onCancel} keys="esc">
          Cancel
        </Button>
        <Button variant="primary" size="small" type="submit" disabled={busy || (!ready && !same)} unavailable={same ? `It reads as ${of} does: say what's decided in its place` : undefined} title={w.send} keys="mod+⏎">
          {busy ? w.sending : w.send}
        </Button>
      </div>
    </form>
  );
}
