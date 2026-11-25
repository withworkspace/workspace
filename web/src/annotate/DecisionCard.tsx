import { useState } from "react";
import type { Thread } from "../api";
import { AnnotateDecision } from "./AnnotateDecision";

type Decide = { action: "record"; what: string; why: string } | { action: "skip" };

// DecisionCard records what was decided in a resolved thread: as its agent
// drafted it (skip it, or edit and record it), or in your own words.
export function DecisionCard({ draft, onDecide, onCancel }: { draft?: NonNullable<Thread["draft"]>; onDecide: (d: Decide) => Promise<void>; onCancel?: () => void }) {
  const [what, setWhat] = useState(draft?.what ?? "");
  const [why, setWhy] = useState(draft?.why ?? "");
  const [busy, setBusy] = useState(false);
  const run = (d: Decide) => {
    if (busy || (d.action === "record" && !d.what.trim())) return;
    setBusy(true);
    onDecide(d).finally(() => setBusy(false));
  };
  return (
    <AnnotateDecision
      draftedBy={draft?.by}
      what={what}
      why={why}
      busy={busy}
      onWhat={setWhat}
      onWhy={setWhy}
      onRecord={() => run({ action: "record", what, why })}
      onSkip={() => run({ action: "skip" })}
      onCancel={onCancel}
    />
  );
}
