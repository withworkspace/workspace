import { type CSSProperties, type ReactNode, useEffect, useRef, useState } from "react";
import { cx } from "../ui/cx";
import type { SidebarPlace } from "./chromeState";
import css from "./Shell.module.css";

type ShellProps = {
  // Where the sidebar is: open, a rail, hidden (it's slid out of the
  // window, and the page has its column), or a drawer, on a narrow window: out of
  // sight until it's opened over the page.
  sidebar: SidebarPlace;
  // The drawer is open, and what closes it: a press on what's behind it.
  drawerOpen?: boolean;
  onDrawerClose?: () => void;
  // The sidebar's card (Sidebar).
  side: ReactNode;
  // Beside the page: a Dock, when there's something to peek at there.
  // Docked, the page makes room for it; over, the page keeps its width
  // and the dock floats over its end. On a narrow window it's a sheet over
  // the page either way, and should be given as one (Dock's sheet).
  beside?: ReactNode;
  besideOver?: boolean;
  // How wide the dock beside is: how far it slides, opening and closing.
  // Without it, it's there at once.
  besideSize?: number;
  // Nothing slides: the window is still opening, and what's beside is
  // there as it was left, not arriving.
  still?: boolean;
  // The foot (Foot), the whole width of the window, under the sidebar
  // too.
  foot?: ReactNode;
  // A plugin's app in the page has a backdrop up: the sidebar and the
  // foot dim with it and take no presses. A press on the dimmed part is
  // said to the app like any press in ws, and takes no focus from the
  // frame, whose dialog still has the keys.
  dim?: boolean;
  // What's over the whole window, not placed in it: dialogs, Find,
  // toasts. They're in the shell so they're on its ground.
  overlays?: ReactNode;
  className?: string;
  // The page's own class: what was last known of a lost daemon is faint.
  pageClassName?: string;
  // The page.
  children: ReactNode;
};

// Shell is the window: the sidebar at its start, the page, what's beside
// it, and the foot along the bottom. It places them and paints the ground
// they're on; what each is, is its own part's. The page scrolls within its
// own room, so the sidebar, what's beside and the foot stay put.
export function Shell({ sidebar, drawerOpen, onDrawerClose, side, beside, besideOver, besideSize, still, foot, dim, overlays, className, pageClassName, children }: ShellProps) {
  const narrow = sidebar === "drawer";
  const drawn = narrow && !!drawerOpen;
  const at = useSliding(beside, !!besideOver, !!still || narrow || !besideSize);
  return (
    <div className={cx(css.shell, sidebar === "rail" && css.railed, sidebar === "hidden" && css.hidden, narrow && css.narrow, drawn && css.drawn, foot ? css.footed : undefined, className)}>
      {drawn && <div className={css.scrim} onClick={onDrawerClose} />}
      {/* Hidden, it's still here, slid out and taking nothing: so it slides
          both ways as the page's column does. */}
      <div className={css.side} inert={dim || sidebar === "hidden" || (narrow && !drawn)}>
        {side}
      </div>
      {dim && sidebar !== "hidden" && !narrow && <div className={cx(css.dim, css.dimSide)} onMouseDown={(e) => e.preventDefault()} />}
      <div className={css.stage}>
        <main id="main" className={cx(css.page, pageClassName)}>
          {children}
        </main>
        {at.node && (
          <div
            className={cx(at.over ? css.besideOver : css.beside, at.moving === "in" && css.arriving, at.moving === "out" && css.leaving)}
            style={{ "--beside-size": `${besideSize ?? 0}px` } as CSSProperties}
            inert={at.moving === "out"}
            onAnimationEnd={(e) => e.target === e.currentTarget && at.settle()}
          >
            <div className={css.besideIn}>{at.node}</div>
          </div>
        )}
      </div>
      {foot && (
        <div className={css.foot} inert={dim}>
          {foot}
        </div>
      )}
      {dim && foot && <div className={cx(css.dim, css.dimFoot)} onMouseDown={(e) => e.preventDefault()} />}
      {overlays}
    </div>
  );
}

// How long a slide may take before it's taken as over, whether or not
// the browser said it ended (it doesn't, of an animation that never ran).
const slideAtMost = 600;

// useSliding is what's beside as it's drawn: it slides in when it opens
// and out when it closes, as the sidebar does, the page's column moving
// with it. While it slides out it's still drawn, as it last was, though
// it's no longer given. Nothing slides while the window is still.
function useSliding(beside: ReactNode, over: boolean, still: boolean): { node: ReactNode; over: boolean; moving?: "in" | "out"; settle: () => void } {
  const last = useRef<{ node: ReactNode; over: boolean }>(undefined);
  const [was, setWas] = useState({ had: !!beside, still });
  const [moving, setMoving] = useState<"in" | "out">();
  if (beside) last.current = { node: beside, over };
  if (was.had !== !!beside || was.still !== still) {
    setWas({ had: !!beside, still });
    if (was.had !== !!beside) setMoving(was.still || still ? undefined : beside ? "in" : "out");
  }
  useEffect(() => {
    if (!moving) return;
    const t = setTimeout(() => setMoving(undefined), slideAtMost);
    return () => clearTimeout(t);
  }, [moving]);
  const settle = () => setMoving(undefined);
  if (beside) return { node: beside, over, moving: moving === "in" ? "in" : undefined, settle };
  if (moving === "out" && last.current) return { ...last.current, moving, settle };
  return { node: undefined, over, settle };
}
