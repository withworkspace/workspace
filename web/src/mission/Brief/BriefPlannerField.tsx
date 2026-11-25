import type { Ref } from "react";
import type { Chip } from "../../ask/ask";
import { AskField } from "../../ask/AskField";

// BriefPlannerField is where the planner's asked: the ask field
// (AskField), with what to ask offered over it as it's gone to, and a
// passage in hand as a chip. placeholder is what it says while empty;
// busy, while what was asked is being sent. What's asked is said in the
// planner's conversation, which is drawn on this page: it's the
// conversation's own view, so nothing opens beside.
export function BriefPlannerField({
  held,
  dropHeld,
  suggestions,
  ask,
  input,
  busy,
  placeholder = "Ask the planner…",
}: {
  held?: Chip & { placeholder: string; drop: string };
  dropHeld: () => void;
  suggestions: string[];
  ask: (text: string) => void;
  input?: Ref<HTMLInputElement>;
  busy: boolean;
  placeholder?: string;
}) {
  return (
    <AskField
      place="inline"
      field={input}
      placeholder={held ? held.placeholder : placeholder}
      busy={busy}
      suggestions={suggestions}
      holding={held && { icon: held.icon, text: held.text, title: held.title, kind: held.kind, drop: { label: held.drop, onDrop: dropHeld } }}
      onAsk={ask}
    />
  );
}
