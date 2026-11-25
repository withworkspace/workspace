import type { PageRef } from "../refs/refs";
import { pinNameOf } from "./pinNames";
import { PinToggle } from "./PinToggle";
import { useFocus } from "./useFocus";

// PinButton pins what a card's about, or unpins it: on a card, beside its
// way there. keyHint says . does it too; compact is the pin alone; end
// keeps it to the end of its line.
export function PinButton({ r, keyHint, compact, end }: { r: PageRef; keyHint?: boolean; compact?: boolean; end?: boolean }) {
  const focus = useFocus();
  const name = pinNameOf(r);
  if (!focus?.supported || !name) return null;
  const on = focus.pinned(name);
  return <PinToggle on={on} compact={compact} end={end} keyHint={keyHint} onToggle={() => (on ? focus.unpin(name) : focus.pin(name))} />;
}
