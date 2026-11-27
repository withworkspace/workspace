import type { KeyLike } from "../keymap/keymap";
import type { PageRef } from "../refs/refs";
import { pinNameOf } from "./pinNames";
import { PinToggle } from "./PinToggle";
import { useFocus } from "./useFocus";

// PinButton pins what a card's about, or unpins it: on a card, beside its
// way there. keys is the key that does the same, where there's one (a
// page's own pin's, Find's for the row the cursor's on); compact is the
// pin alone; end keeps it to the end of its line.
export function PinButton({ r, keys, compact, end }: { r: PageRef; keys?: KeyLike; compact?: boolean; end?: boolean }) {
  const focus = useFocus();
  const name = pinNameOf(r);
  if (!focus?.supported || !name) return null;
  const on = focus.pinned(name);
  return <PinToggle on={on} compact={compact} end={end} keys={keys} onToggle={() => (on ? focus.unpin(name) : focus.pin(name))} />;
}
