import type { ReactNode } from "react";
import { Link } from "react-router";
import { PaneSection } from "../panes/PaneSection";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { PullRequestChecks, type PullRequestChecksState } from "./PullRequestChecks";
import { PullRequestPeekLayer, type PullRequestPeekLayerInfo } from "./PullRequestPeekLayer";
import { PullRequestSize } from "./PullRequestSize";
import css from "./PullRequestPeek.module.css";

export type PullRequestPeekProps = {
  // How it stands with you, and why in a few words.
  status: StatusInfo;
  author: string;
  // It's yours.
  mine?: boolean;
  draft?: boolean;
  // The branch it's from, and the one it's for.
  branch: string;
  base: string;
  additions: number;
  deletions: number;
  files: number;
  checks?: PullRequestChecksState;
  // When it last changed.
  updated?: string;
  // The mission it's the work of, if it's one of this workspace's.
  mission?: { title: string; to: string };
  // The stack it's a layer of, bottom first.
  layers?: PullRequestPeekLayerInfo[];
  // What it says it does: its description, as it's read. None, when it
  // has none.
  children?: ReactNode;
};

// PullRequestPeek is a pull request as it's read beside another page: how
// it stands with you, whose it is and where it's going, how big it is and
// whether its checks pass, the stack it's in, and what it says it does.
// Its diff and your review of it are its own page's, at the pane's head.
export function PullRequestPeek({ status, author, mine, draft, branch, base, additions, deletions, files, checks, updated, mission, layers, children }: PullRequestPeekProps) {
  return (
    <div className={css.peek}>
      <div className={css.stands}>
        <Status s={status} />
        {draft && <Chip tone="quiet">Draft</Chip>}
        <span className={css.by}>
          {mine ? "yours" : `by ${author}`}
          {updated && (
            <>
              {" · "}
              <Time iso={updated} as="relative" />
            </>
          )}
        </span>
      </div>
      <div className={css.facts}>
        <span className={css.branches} title={`${branch} into ${base}`}>
          <Icon name="branch" size={12} />
          <span className={css.branch}>{branch}</span>
          <span className={css.into}>into</span>
          <span className={css.branch}>{base}</span>
        </span>
        <PullRequestSize additions={additions} deletions={deletions} files={files} />
        {checks && <PullRequestChecks checks={checks} />}
      </div>
      {mission && (
        <Link to={mission.to} className={css.mission}>
          <Icon name="flag" size={12} /> The work of {mission.title}
        </Link>
      )}
      {layers && layers.length > 1 && (
        <PaneSection title="Stack" count={layers.length}>
          <ol className={css.layers}>
            {layers.map((l) => (
              <PullRequestPeekLayer key={l.number} {...l} />
            ))}
          </ol>
        </PaneSection>
      )}
      <PaneSection title="What it says it does">
        {children ? <div className={css.says}>{children}</div> : <p className={css.silent}>It has no description.</p>}
      </PaneSection>
    </div>
  );
}
