import type { ReactNode } from "react";
import css from "./Empty.module.css";
import { cx } from "./cx";
import { Icon, type IconName } from "./Icon";

// Empty is a place with nothing in it: what it's for, and what to do.
// Under its icon are a title, a sentence (its children), what's pressed
// (action) and anything said after that (foot). big is one that's a
// page's whole news (nothing's left to approve): its icon in a disc, its
// words a size up; done has the disc in the colour of something done.
// Where it sits is the page's, by the class it passes.
export function Empty({
  icon = "inbox",
  title,
  children,
  action,
  foot,
  big,
  done,
  className,
}: {
  icon?: IconName;
  title: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  foot?: ReactNode;
  big?: boolean;
  done?: boolean;
  className?: string;
}) {
  return (
    <div className={cx(css.empty, big && css.big, className)}>
      {big ? (
        <span className={cx(css.disc, done && css.done)}>
          <Icon name={icon} size={22} />
        </span>
      ) : (
        <Icon name={icon} size={20} className={css.icon} />
      )}
      <div className={css.title}>{title}</div>
      {children && <div className={css.text}>{children}</div>}
      {action && <div className={css.action}>{action}</div>}
      {foot}
    </div>
  );
}
