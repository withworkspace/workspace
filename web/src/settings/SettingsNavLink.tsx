import type { ReactNode } from "react";
import { NavLink, type To } from "react-router";
import { Count } from "../ui/Count";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./SettingsNavLink.module.css";

type SettingsNavLinkProps = {
  to: To;
  icon: IconName;
  // The section's name.
  children: ReactNode;
  // The section you're in.
  current?: boolean;
  // Finding, and nothing found in it: it stands back.
  faded?: boolean;
  // How many: found in it, or set by you.
  count?: number;
  countTitle?: string;
  // Something in the section wants a look: a dot, which says this.
  unread?: string;
  onClick?: () => void;
};

// SettingsNavLink is a section in the Settings page's list: its icon and
// name, and how many of something it has.
export function SettingsNavLink({ to, icon, children, current, faded, count, countTitle, unread, onClick }: SettingsNavLinkProps) {
  return (
    <NavLink to={to} className={() => cx(css.link, current && css.current, faded && css.faded)} aria-current={current ? "page" : undefined} onClick={onClick}>
      <Icon name={icon} size={15} />
      <span>{children}</span>
      {unread && <span className={css.unread} title={unread} />}
      {count !== undefined && (
        <Count className={css.count} title={countTitle}>
          {count}
        </Count>
      )}
    </NavLink>
  );
}
