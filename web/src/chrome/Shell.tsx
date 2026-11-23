import type { ReactNode } from "react";
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

// Shell is the window: the sidebar at its start, the page, and the foot
// along the bottom. It places them and paints the ground they're on; what
// each is, is its own part's. The page scrolls within its own room, so
// the sidebar and the foot stay put.
export function Shell({ sidebar, drawerOpen, onDrawerClose, side, foot, dim, overlays, className, pageClassName, children }: ShellProps) {
  const narrow = sidebar === "drawer";
  const drawn = narrow && !!drawerOpen;
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
