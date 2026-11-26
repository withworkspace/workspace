import { useRef, useState } from "react";
import { noteKinds, type Review, type Verdict } from "../api";
import { useKeys } from "../keys";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Keys } from "../ui/Keys";
import { humanError } from "../ui/strings";
import { TextArea } from "../ui/TextArea";
import css from "./ReviewSubmit.module.css";
import { ReviewVerdicts } from "./ReviewVerdicts";

// ReviewSubmit ends a capsule's review with a verdict: a, c or m picks
// it, then the summary, then ⌘⏎. It says what's in the draft, and can
// discard it.
export function ReviewSubmit({
  draft,
  onClose,
  onDiscard,
  onSubmit,
}: {
  draft: Review | null;
  onClose: () => void;
  onDiscard: () => void;
  onSubmit: (v: Verdict, summary: string) => Promise<void>;
}) {
  const [verdict, setVerdict] = useState<Verdict>();
  const [summary, setSummary] = useState("");
  const [error, setError] = useState<string>();
  const area = useRef<HTMLTextAreaElement>(null);
  const pick = (v: Verdict) => {
    setVerdict(v);
    setTimeout(() => area.current?.focus());
  };
  const submit = () => {
    if (!verdict) return setError("Pick a verdict first.");
    onSubmit(verdict, summary).catch((e: Error) => setError(e.message));
  };
  useKeys({ a: () => pick("approve"), c: () => pick("changes"), m: () => pick("comment") }, { modal: true });
  const counts = noteKinds.flatMap((k) => {
    const n = draft?.comments.filter((c) => c.kind === k).length ?? 0;
    return n ? [`${n} ${k}`] : [];
  });
  return (
    <Dialog title="Submit review" onClose={onClose} className={css.dialog} wide>
      <p>
        {draft
          ? `${draft.comments.length} new ${draft.comments.length === 1 ? "comment" : "comments"}${counts.length ? ` (${counts.join(", ")})` : ""}.`
          : "No new comments."}{" "}
        Once submitted, crew read it.
      </p>
      <ReviewVerdicts label="Verdict" value={verdict} keyed onPick={pick} />
      <TextArea
        ref={area}
        value={summary}
        rows={4}
        placeholder="Summary (optional)"
        aria-label="Summary"
        onChange={(e) => setSummary(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            submit();
          } else if (e.key === "Escape") {
            e.preventDefault();
            onClose();
          }
        }}
      />
      {error && <Banner>{humanError(error)}</Banner>}
      <div className={css.actions}>
        {draft && (
          <Button variant="ghost" onClick={onDiscard}>
            Discard draft
          </Button>
        )}
        <span className={css.spacer} />
        <Button variant="ghost" onClick={onClose} keys="esc">
          Cancel
        </Button>
        <Button variant="primary" onClick={submit} unavailable={verdict ? undefined : "Pick a verdict first"} title="Submit" keys="mod+⏎">
          Submit <Keys k="mod+⏎" />
        </Button>
      </div>
    </Dialog>
  );
}
