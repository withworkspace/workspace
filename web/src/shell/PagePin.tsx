import type { PageRef } from "../refs/refs";
import { PinButton } from "./PinButton";

// PagePin is a page's own Pin, in its head: PinButton, for what the page is.
// compact is the pin alone, its words in its title: where a head's row
// has no room for more. end keeps it to the end of the head's line.
export function PagePin({ r, compact, end }: { r: PageRef; compact?: boolean; end?: boolean }) {
  return <PinButton r={r} keys="." compact={compact} end={end} />;
}
