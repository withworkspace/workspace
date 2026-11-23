import type { ReactNode } from "react";
import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import css from "./PaneHead.module.css";

type PaneHeadProps = {
  // The thing's title, where its tab doesn't already say it whole. With
  // none, the head is the one quiet line: how it stands, and the ways on.
  title?: string;
  // Its own page, which Open goes to in the page's place.
  to?: string;
  // How it stands, where it is: under the title, or the line itself.
  sub?: ReactNode;
  // Before Open: what else can be done with it (Keep).
  more?: ReactNode;
};

// PaneHead is the head of a thing shown in a dock: how it stands, and the
// way to its own page. Its tab names it; the head says it again only
// where the tab can't say all of it.
export function PaneHead({ title, to, sub, more }: PaneHeadProps) {
  const ways = (
    <>
      {more}
      {to && (
        <Link to={to} className={css.open} title="Go to its own page, in the page's place">
          Open <Icon name="arrow-up-right" size={12} />
        </Link>
      )}
    </>
  );
  if (title === undefined) {
    return (
      <header className={css.head}>
        <div className={css.line}>
          <div className={`${css.sub} ${css.only}`}>{sub}</div>
          {ways}
        </div>
      </header>
    );
  }
  return (
    <header className={css.head}>
      <div className={css.line}>
        <h3 className={css.title} title={title}>
          {title}
        </h3>
        {ways}
      </div>
      {sub && <div className={css.sub}>{sub}</div>}
    </header>
  );
}
