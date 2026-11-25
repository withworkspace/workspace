import type { ReactNode } from "react";
import { Icon } from "../../ui/Icon";
import css from "./MissionWorkLeft.module.css";

// MissionWorkLeft is what a crew left, under its line: its handoff, in a
// line or two, which opens to be read (onOpen), or the last thing it said.
export function MissionWorkLeft({ children, onOpen, title }: { children: ReactNode; onOpen?: () => void; title?: string }) {
  if (!onOpen) {
    return (
      <div className={css.left}>
        <span>{children}</span>
      </div>
    );
  }
  return (
    <button className={css.left} onClick={onOpen} title={title}>
      <Icon name="arrow-right" size={12} />
      <span>{children}</span>
    </button>
  );
}
