import type { CodeHit } from "../find";
import { FindCodeHit, findCodeHitClass } from "./FindCodeHit";

// FindCodeRow is a line a file says, among what Find found: a FindCodeHit
// to pick, which says its repo and its capsule (Find looks through all of
// them). on, onOver and onPick are a FindRow's.
export function FindCodeRow({ id, on, h, onOver, onPick }: { id: string; on?: boolean; h: CodeHit; onOver?: () => void; onPick?: (tab: boolean) => void }) {
  return (
    <div id={id} role="option" aria-selected={!!on} className={findCodeHitClass} onMouseMove={onOver} onClick={(e) => onPick?.(e.metaKey || e.ctrlKey)}>
      <FindCodeHit h={h} repo capsule />
    </div>
  );
}
