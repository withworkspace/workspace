import type { ReactNode } from "react";
import { Link, type To } from "react-router";
import { Count } from "../../ui/Count";
import { Icon, type IconName } from "../../ui/Icon";
import css from "./FilesTabs.module.css";

// FilesTabs are a worktree's tabs, on its bar: its Files, its Runs, and
// each of its apps. Underlined, as Tabs are, and standing on the bar's own
// line.
export function FilesTabs({ children }: { children: ReactNode }) {
  return (
    <nav className={css.tabs} aria-label="Tabs">
      {children}
    </nav>
  );
}

// FilesTab is one of them: a link to it, the one shown underlined, with
// how many of what it holds are going now (count).
export function FilesTab({ to, icon, label, count, current }: { to: To; icon: IconName; label: string; count?: number; current?: boolean }) {
  return (
    <Link to={to} className={css.tab} aria-current={current ? "page" : undefined} replace>
      <Icon name={icon} size={14} />
      {label}
      {!!count && <Count className={css.count}>{count}</Count>}
    </Link>
  );
}
