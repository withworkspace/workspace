import { Fragment, type ReactNode, type Ref } from "react";
import { Link } from "react-router";
import css from "./Crumbs.module.css";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { keyProps } from "./keyHint";

type CrumbsProps = {
  // Where the page is in, nearest first: the first is the way back, with
  // an arrow; any others are beside it, a dot between.
  // A step's keys is the key that goes there too (esc, for the way back):
  // said in its title, and hinted at (ui/keyHint.ts).
  steps?: { to: string; label: ReactNode; keys?: string }[];
  // Or what a path of its own is made of (links, menus, CrumbSeps).
  children?: ReactNode;
  // The page's own class, to place it.
  className?: string;
  ref?: Ref<HTMLElement>;
};

// Crumbs is where a page is in, over its head, small and quiet: "← the
// mission", and anywhere else it's reached from. It's the way back on a
// page with no bar of its own (crew, an artifact, a run, a review).
export function Crumbs({ steps, children, className, ref }: CrumbsProps) {
  return (
    <nav className={cx(css.crumbs, className)} aria-label="Breadcrumb" data-crumbs="" ref={ref}>
      {steps?.map((s, i) => (
        <Fragment key={i}>
          {i > 0 && <CrumbSep />}
          <Link to={s.to} {...(s.keys ? keyProps(s.keys) : {})}>
            {i === 0 && <Icon name="arrow-left" size={13} />} {s.label}
          </Link>
        </Fragment>
      ))}
      {children}
    </nav>
  );
}

// CrumbSep is what's between two steps: a dot, or a path's slash.
export function CrumbSep({ children = "·", className }: { children?: ReactNode; className?: string }) {
  return <span className={cx(css.sep, className)}>{children}</span>;
}
