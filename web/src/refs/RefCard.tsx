import { PinButton } from "../shell/focus";
import type { Anchor } from "../ui/FloatingCard";
import { useWorkspace } from "../workspace/store";
import { useRefs } from "./known";
import type { PageRef } from "./refs";
import { refBody } from "./RefCardBody";
import { RefFloat } from "./RefFloat";

// RefCard is the card for a ref, under the words that name it (el), or
// over them where there's no room below. The words are an element of the
// page's, or, for a ref in a plugin's page, where the page said they are
// (plugins/ui.ts). hint says ⌘-click goes there:
// where words are written, a click is for the cursor.
export function RefCard({ r, el, onEnter, onLeave, hint = true }: { r: PageRef; el: Anchor; onEnter: () => void; onLeave: () => void; hint?: boolean }) {
  const workspace = useWorkspace();
  // Read here, outside the card: inside it, names are only words.
  const refs = useRefs();
  return (
    <RefFloat anchor={el} onEnter={onEnter} onLeave={onLeave} hint={hint} pin={<PinButton r={r} />}>
      {refBody(r, workspace, refs?.known, refs?.repos)}
    </RefFloat>
  );
}
