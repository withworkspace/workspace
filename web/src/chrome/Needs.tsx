import type { ReactNode } from "react";
import { Link } from "react-router";
import { dealtWithAt, type Need, toReview } from "./need";
import { NeedRow } from "./NeedRow";
import css from "./Needs.module.css";
import { SidebarSection } from "./SidebarSection";
import { SidebarSubhead } from "./SidebarSubhead";

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
  // The same for the pull requests that ask for your review: they're
  // other people's work, as many as other people make them, so only a
  // few are listed here and Reviews has the rest.
  mostToReview?: number;
  moreToReview?: string;
  // Beside its heading: a setting of the list's own.
  tool?: ReactNode;
  // A row's menu, opened on it with the other button: asked for by
  // where the row goes and what it's called.
  menu?: (to: string, title?: string) => React.MouseEventHandler<Element> | undefined;
  className?: string;
};

// Needs is Needs you, in the sidebar: what can't move without you, a
// NeedRow each. Your own come first (missions, conversations); the pull
// requests that ask for your review are under a small heading of their
// own, after them, so someone else's work never puts crew's question
// further down. It's not there when nothing needs you. In the rail its
// heading is a rule and its rows their icons.
export function Needs({
  needs,
  path,
  fresh,
  most = 10,
  moreTo = "/",
  mostToReview = 4,
  moreToReview = "/reviews",
  tool,
  menu,
  className,
}: NeedsProps) {
  if (!needs.length) return null;
  const yours = needs.filter((n) => !toReview(n));
  const reviews = needs.filter(toReview);
  const rows = (list: Need[], few: number, to: string) => (
    <>
      {list.slice(0, few).map((n) => (
        <NeedRow
          key={n.key}
          need={n}
          on={dealtWithAt(n, path)}
          fresh={fresh?.has(n.key)}
          onContextMenu={menu?.(n.to, n.title)}
        />
      ))}
      {list.length > few && (
        <Link to={to} className={css.more}>
          {list.length - few} more
        </Link>
      )}
    </>
  );
  return (
    <SidebarSection label="Needs you" tool={tool} className={className}>
      {rows(yours, most, moreTo)}
      {reviews.length > 0 && (
        <>
          <SidebarSubhead label="To review" />
          {rows(reviews, mostToReview, moreToReview)}
        </>
      )}
    </SidebarSection>
  );
}
