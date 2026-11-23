import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./Sidebar.module.css";

type SidebarProps = {
  // Folded to a rail, on a page that wants the room: a column of icons,
  // which opens over the page while it's pointed at or the keys are in
  // it. Nothing in it is ever hidden.
  rail?: boolean;
  className?: string;
  children: ReactNode;
};

// Sidebar is the workspace's card at the window's start: its name, Find,
// its places, and what needs you (WorkspaceName, FindButton, Places,
// Needs). It's drawn open or as a rail, or not at all when you've hidden
// it (the foot's first toggle).
//
// What's in it reads data-sidebar to know which it is, since the rail
// opening is the pointer's doing and only CSS knows of it.
export function Sidebar({ rail, className, children }: SidebarProps) {
  return (
    <aside
      aria-label="Workspace"
      data-sidebar={rail ? "rail" : "open"}
      className={cx(css.sidebar, rail && css.rail, className)}
    >
      {children}
    </aside>
  );
}
