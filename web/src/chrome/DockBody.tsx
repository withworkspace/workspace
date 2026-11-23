import { type ReactNode, useCallback, useRef } from "react";
import { cx } from "../ui/cx";
import { ScrollerContext } from "../ui/scroller";
import css from "./DockBody.module.css";

type DockBodyProps = {
  // What it shows, for a screen reader: the open tab's name.
  label: string;
  // Set in from the dock's edges: for what's read (a conversation, a
  // document). A list of rows, or a terminal, runs to the edges without.
  pad?: boolean;
  className?: string;
  children: ReactNode;
};

// DockBody is what a dock's open tab shows. It takes the room the dock
// has left and scrolls within it: what's drawn in it that follows its
// scroll (a conversation keeping to its latest) follows this, not the
// page (ui/scroller.ts).
export function DockBody({ label, pad, className, children }: DockBodyProps) {
  const body = useRef<HTMLDivElement>(null);
  const scroller = useCallback(() => body.current, []);
  return (
    <div
      ref={body}
      role="tabpanel"
      aria-label={label}
      className={cx(css.body, pad && css.pad, className)}
    >
      <ScrollerContext.Provider value={scroller}>
        {children}
      </ScrollerContext.Provider>
    </div>
  );
}
