import { type KeyLike, useSaid } from "../../keymap/keymap";
import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { keyProps } from "../../ui/keyHint";
import css from "./FindTag.module.css";

// FindTag is said before what's typed in Find: what it's narrowed to (a
// kind, a repo), or, with doing, what it's in the middle of (the thing
// it's acting on, the action that's asking), which is cut short where
// it's long. A press takes it away, and leaves the focus in the text box.
export function FindTag({ doing, title, keys: given, onClick, children }: { doing?: boolean; title: string; keys?: KeyLike; onClick: () => void; children: ReactNode }) {
  const keys = useSaid(given);
  return (
    <button className={cx(css.tag, doing && css.doing)} onMouseDown={(e) => e.preventDefault()} {...keyProps(keys, title, () => onClick())}>
      {children} <Icon name="x" size={10} />
    </button>
  );
}
