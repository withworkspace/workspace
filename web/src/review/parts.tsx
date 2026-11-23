import { useRef, useState } from "react";
import { noteKinds, type Review, type Verdict } from "../api";
import { useKeys } from "../keys";
import { Icon } from "../ui/Icon";
import { humanError, Keys } from "../ui/kit";
import { Dialog } from "../ui/Dialog";
import { Button } from "../ui/Button";
import { TextArea } from "../ui/TextArea";

// What a capsule's review and a document's share: how verdicts and comment
// states read, suggestions as written in a comment, and submitting.

export const verdictWords: Record<Verdict, string> = { approve: "approved", changes: "changes requested", comment: "comments" };

export const statusWords: Record<string, string> = { open: "Open", addressed: "Addressed", wontfix: "Won't fix", resolved: "Resolved" };

// A suggestion is written as a ```suggestion block in the comment, as on
// GitHub; it's kept apart from the words.
export const suggestionBlock = /```suggestion\n([\s\S]*?)\n?```\n?/;

export function splitSuggestion(text: string): { body: string; suggestion: string | null } {
  const m = text.match(suggestionBlock);
  if (!m) return { body: text.trim(), suggestion: null };
  return { body: text.replace(suggestionBlock, "").trim(), suggestion: m[1] };
}

export function joinSuggestion(body: string, suggestion?: string | null) {
  return suggestion == null ? body : `${body}${body ? "\n\n" : ""}\`\`\`suggestion\n${suggestion}\n\`\`\``;
}

// SubmitDialog ends the review with a verdict: a, c or m picks it, then
// the summary, then ⌘⏎.
export function SubmitDialog({
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
  const choices: [Verdict, string, string, "check-circle" | "alert" | "message"][] = [
    ["approve", "a", "Approve", "check-circle"],
    ["changes", "c", "Request changes", "alert"],
    ["comment", "m", "Comment", "message"],
  ];
  return (
    <Dialog title="Submit the review" onClose={onClose} className="submit-dialog" wide>
        <p>
          {draft
            ? `${draft.comments.length} new ${draft.comments.length === 1 ? "comment" : "comments"}${counts.length ? ` (${counts.join(", ")})` : ""}.`
            : "No new comments."}{" "}
          Once it's submitted, crew read it and the orchestrator decides what follows.
        </p>
        <div className="verdicts" role="radiogroup" aria-label="Verdict">
          {choices.map(([v, key, label, icon]) => (
            <button key={v} role="radio" aria-checked={verdict === v} className={`verdict v-${v}${verdict === v ? " on" : ""}`} onClick={() => pick(v)}>
              <Icon name={icon} size={14} /> {label} <Keys k={key} />
            </button>
          ))}
        </div>
        <TextArea
          ref={area}
          value={summary}
          rows={4}
          placeholder="A summary, if it helps (optional)"
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
        {error && <div className="banner">{humanError(error)}</div>}
        <div className="note-actions">
          {draft && (
            <Button variant="ghost" onClick={onDiscard}>
              Discard the draft
            </Button>
          )}
          <span className="spacer-left" />
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} unavailable={verdict ? undefined : "Pick a verdict first"} title="Submit (⌘⏎)">
            Submit <Keys k="mod+⏎" />
          </Button>
        </div>
    </Dialog>
  );
}

