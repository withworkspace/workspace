import type { ReactNode } from "react";
import { Link } from "react-router";
import { hoverCardLinkClass } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";

// RefGo is where a ref's card goes, at its foot: in ws (to), or outside,
// in a tab of its own (href).
export function RefGo({ to, href, children }: { to?: string; href?: string; children: ReactNode }) {
  if (href !== undefined)
    return (
      <a href={href} target="_blank" rel="noreferrer" className={hoverCardLinkClass()}>
        {children} <Icon name="external" size={11} />
      </a>
    );
  return (
    <Link to={to ?? ""} className={hoverCardLinkClass()}>
      {children} <Icon name="arrow-right" size={11} />
    </Link>
  );
}
