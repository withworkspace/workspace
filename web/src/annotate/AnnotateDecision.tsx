import { Button } from "../ui/Button";
import { AnnotateSuggestion } from "./AnnotateSuggestion";
import { RichTextArea } from "../ui/RichTextArea";

const rows = (s: string) => Math.min(5, Math.max(2, Math.ceil(s.length / 40)));

// AnnotateDecision is what was decided in a resolved thread, to record: as
// its agent drafted it (draftedBy: skip it, or edit and record it), or in
// your own words (Cancel). ⌘⏎ in either box records it; busy, it's being
// sent.
export function AnnotateDecision({
  draftedBy,
  what,
  why,
  busy,
  onWhat,
  onWhy,
  onRecord,
  onSkip,
  onCancel,
}: {
  draftedBy?: string;
  what: string;
  why: string;
  busy?: boolean;
  onWhat: (what: string) => void;
  onWhy: (why: string) => void;
  onRecord: () => void;
  onSkip?: () => void;
  onCancel?: () => void;
}) {
  // ⌘⏎ records it, whatever sends elsewhere: it's a form of two boxes.
  const keys = { send: "mod+enter", onSend: onRecord } as const;
  const drafted = draftedBy !== undefined;
  return (
    <AnnotateSuggestion
      kind="decision"
      icon="gavel"
      title={drafted ? "Record decision?" : "Decision"}
      by={drafted && `drafted by ${draftedBy === "agent" ? "the agent" : draftedBy}`}
      actions={
        <>
          {drafted ? (
            <Button variant="ghost" size="small" disabled={busy} onClick={onSkip}>
              Skip
            </Button>
          ) : (
            <Button variant="ghost" size="small" onClick={onCancel}>
              Cancel
            </Button>
          )}
          <Button variant="primary" size="small" disabled={busy || !what.trim()} onClick={onRecord} title="Record" keys="mod+⏎">
            Record
          </Button>
        </>
      }
    >
      <RichTextArea refs look="field" value={what} rows={rows(what)} placeholder="The decision, in a sentence" aria-label="Decision" onChange={onWhat} {...keys} />
      <RichTextArea marks lists quote code refs look="field" value={why} rows={rows(why)} placeholder="Why" aria-label="Why" onChange={onWhy} {...keys} />
    </AnnotateSuggestion>
  );
}
