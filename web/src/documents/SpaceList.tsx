import type { ReactNode, Ref } from "react";
import css from "./SpaceList.module.css";

// SpaceList is a space's list: what's over it (top: its search, what's
// happening in it, its tabs) and its lists, which scroll by themselves
// (wide), told when they do.
export function SpaceList({ top, scrollRef, onScroll, children }: { top: ReactNode; scrollRef: Ref<HTMLDivElement>; onScroll: () => void; children: ReactNode }) {
  return (
    <div className={css.list} data-space-list="">
      <div className={css.top}>{top}</div>
      <div className={css.scroll} ref={scrollRef} onScroll={onScroll}>
        {children}
      </div>
    </div>
  );
}
