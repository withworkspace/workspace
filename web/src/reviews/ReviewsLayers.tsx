import type { ReactNode } from "react";
import { Link } from "react-router";
import type { PR } from "../api";
import { Status, type StatusInfo } from "../ui/Status";
import { PullRequestCheckMark } from "./PullRequestCheckMark";
import css from "./ReviewsLayers.module.css";
import { Text } from "../ui/Text";
import { Spacer } from "../ui/Spacer";

// ReviewsLayer is one of a stack's pull requests: where it stands, its
// title and number, what it asks of you (words), its passes, how much it changes
// and its checks. It opens here (to), or on GitHub (href).
export interface ReviewsLayer {
  number: number;
  status: StatusInfo;
  title: string;
  words: string; // what it asks of you
  passes?: ReactNode;
  additions: number;
  deletions: number;
  checks?: PR["checks"];
  to?: string;
  href?: string;
}

// ReviewsLayers are a stack's pull requests, bottom first: the order to
// read them in, hung off its row. Each opens its own review.
export function ReviewsLayers({ layers }: { layers: ReviewsLayer[] }) {
  return (
    <ol className={css.layers}>
      {layers.map((l) => {
        const inner = (
          <>
            <Status s={l.status} iconOnly size={13} />
            <span className={css.title}>{l.title}</span>
            <Text mono dim>#{l.number}</Text>
            <span className={`${css.words} tone-${l.status.tone}`}>{l.words}</span>
            {l.passes}
            <Spacer />
            <span className={css.size}>
              <span className={css.plus}>+{l.additions}</span> <span className={css.minus}>−{l.deletions}</span>
            </span>
            <PullRequestCheckMark checks={l.checks} />
          </>
        );
        return (
          <li key={l.number}>
            {l.to ? (
              <Link to={l.to} className={css.link}>
                {inner}
              </Link>
            ) : (
              <a href={l.href} target="_blank" rel="noreferrer" className={css.link}>
                {inner}
              </a>
            )}
          </li>
        );
      })}
    </ol>
  );
}
