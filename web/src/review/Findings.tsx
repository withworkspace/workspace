import { type DismissWhy, type PassFinding } from "../api";
import { useKeys } from "../keys";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Keys } from "../ui/kit";
import type { Found, Row } from "./model";
import { Button } from "../ui/Button";
import { NoteBody, NoteCard, NoteHead } from "../ui/NoteCard";

// What a pass found, in the diff: a card under the lines it's about, like
// a comment's, but nobody's said it yet. Taking it puts it in your draft
// as a comment; dismissing it says why.

// Why a review's finding is dismissed: what a page's is, as a reviewer
// would say it.
export const reviewDismissWhys: [DismissWhy, string, string][] = [
  ["wrong", "Wrong", "It isn't so"],
  ["already", "Said already", "Someone's said it, or it's known"],
  ["minor", "Not worth it", "It's so, but not worth raising"],
];

// What a follow-up says of a comment.
export const followWords: Record<string, string> = { addressed: "Dealt with", partly: "Partly dealt with", "not-addressed": "Not dealt with" };

// What's done with a finding: taken into your draft, dismissed (and why),
// or asked about.
export type FindingAct = (found: { pass: string; finding: PassFinding }, what: "taken" | "dismissed" | "open" | "ask", why?: DismissWhy) => void;

const linesWord = (f: PassFinding) => (f.endLine && f.line && f.endLine > f.line ? `lines ${f.line}–${f.endLine}` : `line ${f.line}`);

// DismissAs asks why a finding is dismissed: 1, 2 or 3, or a click.
export function DismissAs({ onDismiss, onCancel }: { onDismiss: (why: DismissWhy) => void; onCancel: () => void }) {
  useKeys(
    {
      "1": () => onDismiss("wrong"),
      "2": () => onDismiss("already"),
      "3": () => onDismiss("minor"),
      Escape: onCancel,
    },
    { modal: true },
  );
  return (
    <span className="dismiss-as" role="group" aria-label="Dismiss it as" onClick={(e) => e.stopPropagation()}>
      <span className="dim">Dismiss as</span>
      {reviewDismissWhys.map(([why, label, title], i) => (
        <Button variant="ghost" size="small" key={why} title={title} onClick={() => onDismiss(why)}>
          {label} <Keys k={`${i + 1}`} />
        </Button>
      ))}
      <Button variant="ghost" size="small" square onClick={onCancel} aria-label="Don't dismiss it" title="Don't dismiss it (esc)">
        <Icon name="x" size={11} />
      </Button>
    </span>
  );
}

// FindingActions are what's done with an open finding, as buttons.
export function FindingActions({
  dismissing,
  cursor,
  canAsk,
  onTake,
  onDismissing,
  onDismiss,
  onAsk,
}: {
  dismissing: boolean;
  cursor?: boolean; // the cursor's on it: say its keys
  canAsk?: boolean;
  onTake: () => void;
  onDismissing: (on: boolean) => void;
  onDismiss: (why: DismissWhy) => void;
  onAsk?: () => void;
}) {
  if (dismissing) return <DismissAs onDismiss={onDismiss} onCancel={() => onDismissing(false)} />;
  return (
    <span className="finding-actions" onClick={(e) => e.stopPropagation()}>
      <Button variant="primary" size="small" onClick={onTake} title="Put it in your draft review as a comment, marked as the agent's: yours to edit, and posted only when you submit (⏎)">
        Take {cursor && <Keys k="⏎" />}
      </Button>
      <Button variant="ghost" size="small" onClick={() => onDismissing(true)} title="It isn't one: don't raise it again. You'll say why (D)">
        Dismiss {cursor && <Keys k="D" />}
      </Button>
      {canAsk && onAsk && (
        <Button variant="ghost" size="small" onClick={onAsk} title="Ask an agent about it, with the code checked out here (a)">
          Ask {cursor && <Keys k="a" />}
        </Button>
      )}
    </span>
  );
}

// SuggestedLines is the code a finding would put in place of its lines.
export function SuggestedLines({ from, to }: { from?: string[]; to: string }) {
  return (
    <div className="suggestion">
      <div className="dim">Suggested change</div>
      {(from ?? []).map((l, i) => (
        <div key={`d${i}`} className="sl d">
          − {l}
        </div>
      ))}
      {to.split("\n").map((l, i) => (
        <div key={`a${i}`} className="sl a">
          + {l}
        </div>
      ))}
    </div>
  );
}

// FindingCard is a finding under its lines.
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
  const { finding: f, title, pass }: Found = row.found;
  const on = { pass, finding: f };
  return (
    <NoteCard id={`row-${row.key}`} className="diff-note" found blocks={f.blocks} cursor={cursor} onClick={() => onRow(row.key)}>
      <NoteHead
        end={
          <FindingActions
            dismissing={dismissing}
            cursor={cursor}
            canAsk={canAsk}
            onTake={() => onAct(on, "taken")}
            onDismissing={(v) => onDismissing(v ? row.key : undefined)}
            onDismiss={(why) => onAct(on, "dismissed", why)}
            onAsk={() => onAct(on, "ask")}
          />
        }
      >
        <span className="finding-pass" title={`Found by the ${title} pass. Nobody's said it: take it into your review, or dismiss it`}>
          <Icon name="sparkle" size={11} /> {title}
        </span>
        <span className={`kind k-found${f.blocks ? " k-blocking" : ""}`} title={f.blocks ? "A finding of this kind blocks: the pass is no go while it's open" : undefined}>
          {f.kind}
        </span>
        <span className="dim note-where">
          {f.side === "old" ? "removed " : ""}
          {linesWord(f)}
        </span>
      </NoteHead>
      <NoteBody>
        <Inline text={f.text} />
      </NoteBody>
      {f.suggest ? <SuggestedLines from={f.excerpt} to={f.suggest} /> : null}
    </NoteCard>
  );
}
