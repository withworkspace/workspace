import { Link } from "react-router";
import { cx } from "../ui/cx";
import { Status, type StatusInfo } from "../ui/Status";
import css from "./PullRequestPeekLayer.module.css";

export type PullRequestPeekLayerInfo = {
  number: number;
  title: string;
  // How it stands with you.
  status: StatusInfo;
  // Its review here.
  to: string;
  // It's the one being peeked at.
  here?: boolean;
};

// PullRequestPeekLayer is one layer of the stack a pull request is in:
// how it stands, its number and what it's called, as the way to its own
// review. The one that's being looked at says so.
export function PullRequestPeekLayer({ number, title, status, to, here }: PullRequestPeekLayerInfo) {
  return (
    <li className={css.layer}>
      <Link to={to} className={cx(css.link, here && css.here)} aria-current={here ? "true" : undefined}>
        <Status s={status} iconOnly size={13} />
        <span className={css.number}>#{number}</span>
        <span className={css.title}>{title}</span>
      </Link>
    </li>
  );
}
