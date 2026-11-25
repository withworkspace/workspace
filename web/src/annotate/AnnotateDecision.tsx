import type { KeyboardEvent } from "react";
import { Button } from "../ui/Button";
import { TextArea } from "../ui/TextArea";
import { AnnotateSuggestion } from "./AnnotateSuggestion";

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
  const keys = (e: KeyboardEvent) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      onRecord();
    }
  };
  const drafted = draftedBy !== undefined;
  return (
    <AnnotateSuggestion
      kind="decision"
      icon="gavel"
      title={drafted ? "Record what was decided?" : "What was decided"}
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
          <Button variant="primary" size="small" disabled={busy || !what.trim()} onClick={onRecord} title="Record it" keys="mod+⏎">
            Record
          </Button>
        </>
      }
    >
      <TextArea value={what} rows={rows(what)} placeholder="The decision, in a sentence" onChange={(e) => onWhat(e.target.value)} onKeyDown={keys} />
      <TextArea value={why} rows={rows(why)} placeholder="Why" onChange={(e) => onWhy(e.target.value)} onKeyDown={keys} />
    </AnnotateSuggestion>
  );
}
