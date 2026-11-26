import type { ReactNode } from "react";
import { Icon, type IconName } from "../ui/Icon";
import css from "./AppBar.module.css";
import { Text } from "../ui/Text";

type AppBarProps = {
  // The app's icon and name: a workspace app's bar says them; a
  // worktree's tab already names it.
  icon?: IconName;
  title?: string;
  // What its needs are, or are pointed at: AppBarNeeds, AppBarPointers.
  children?: ReactNode;
  // At its end: how it stands, and what's pressed.
  end: ReactNode;
};

// AppBar is the slim bar over an app: its name, its needs, then how it
// stands and what's done to it.
export function AppBar({ icon, title, children, end }: AppBarProps) {
  return (
    <header className={css.bar}>
      {icon && <Icon name={icon} className={css.icon} />}
      {title && <h1 className={css.title}>{title}</h1>}
      {children}
      <span className={css.spacer} />
      {end}
    </header>
  );
}

// AppBarNeed is a need of a worktree's app, and what met it.
export function AppBarNeed({ name, from, title }: { name: string; from?: string; title: string }) {
  return (
    <span className={css.need} title={title}>
      <span className={css.name}>{name}</span> <Text dim>from</Text> {from}
    </span>
  );
}

// AppBarPointer is a need of a workspace app, and what it's pointed at
// (children: the Select to point it).
export function AppBarPointer({ name, title, children }: { name: string; title: string; children: ReactNode }) {
  return (
    <span className={css.pointer} title={title}>
      <span className={css.name}>{name}</span>
      {children}
    </span>
  );
}

// AppBarLink opens the app in a tab of its own.
export function AppBarLink({ href }: { href: string }) {
  return (
    <a className={css.link} href={href} target="_blank" rel="noreferrer" title={`Open ${href} in new tab`} aria-label="Open in new tab">
      <Icon name="external" size={15} />
    </a>
  );
}
