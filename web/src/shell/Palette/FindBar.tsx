import type { ReactNode } from "react";
import { finderInput } from "../../ui/Finder";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";

// FindBar is the row Find's text box is in, as a finder's: before the
// box, what it's narrowed to or in the middle of (tags, FindTags), and
// after it, that it's still looking (busy).
export function FindBar({ tags, busy, children }: { tags?: ReactNode; busy?: boolean; children: ReactNode }) {
  return (
    <div className={finderInput}>
      <Icon name="search" size={16} />
      {tags}
      {children}
      {busy && <Spinner size={14} />}
    </div>
  );
}
