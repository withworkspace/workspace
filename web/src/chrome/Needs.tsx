import type { ReactNode } from "react";
import { Link } from "react-router";
import { dealtWithAt, type Need } from "./need";
import { NeedRow } from "./NeedRow";
import css from "./Needs.module.css";
import { SidebarSection } from "./SidebarSection";

type NeedsProps = {
  needs: Need[];
  // The page you're on: the need it deals with is marked.
  path: string;
  // The needs that have just arrived, by key (useArrived): each row
  // lights up once.
  fresh?: Set<string>;
  // How many are listed before the rest are a line saying how many more,
  // and where that goes.
  most?: number;
  moreTo?: string;
  // Beside its heading: a setting of the list's own.
  tool?: ReactNode;
  className?: string;
};

// Needs is Needs you, in the sidebar: what can't move without you, a
// NeedRow each. It's not there when nothing does. In the rail its heading
// is a rule and its rows their icons.
export function Needs({
  needs,
  path,
  fresh,
  most = 10,
  moreTo = "/",
  tool,
  className,
}: NeedsProps) {
  if (!needs.length) return null;
  const more = needs.length - most;
  return (
    <SidebarSection label="Needs you" tool={tool} className={className}>
      {needs.slice(0, most).map((n) => (
        <NeedRow
          key={n.key}
          need={n}
          on={dealtWithAt(n, path)}
          fresh={fresh?.has(n.key)}
        />
      ))}
      {more > 0 && (
        <Link to={moreTo} className={css.more}>
          {more} more
        </Link>
      )}
    </SidebarSection>
  );
}
