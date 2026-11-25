import type { ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import css from "./MissionSideCameFrom.module.css";

// MissionSideCameFrom is the conversation a mission came out of, in its
// details: a link to it, by its name.
export function MissionSideCameFrom({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link to={to} className={cx("link", css.from)}>
      <Icon name="message" size={12} />
      {children}
    </Link>
  );
}
