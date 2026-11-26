import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Checkbox } from "../ui/Checkbox";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { keyProps } from "../ui/keyHint";
import { capital } from "../ui/strings";
import css from "./ReviewFile.module.css";
import { Text } from "../ui/Text";

// How a file's changed, in its colour: modified is the chip's own.
const statusClass: Record<string, string | undefined> = { added: css.added, deleted: css.deleted, renamed: css.renamed, copied: css.copied };

// ReviewFile is a file in a review: its head (how it's changed, its path,
// the lines added and taken away, whether it's folded as unchanged since
// you reviewed it or as one to skim, and Viewed, x), stuck to the top
// while it's read; under it what the guide says of it, then its lines.
// Viewed, or folded, it's its head alone.
export function ReviewFile({
  index,
  status,
  path,
  oldPath,
  additions,
  deletions,
  viewed,
  folded,
  cursor,
  unchanged,
  skim,
  guide,
  onHead,
  onFold,
  onViewed,
  children,
}: {
  index: number; // where it is in the review: its id, and its head's
  status: string;
  path: string;
  oldPath?: string;
  additions: number;
  deletions: number;
  viewed: boolean;
  folded: boolean;
  cursor: boolean; // the cursor's on its head
  unchanged?: boolean; // its change is as you reviewed it
  skim?: string; // why it's one to skim
  guide?: string; // what the change does to it, in a line (a guide's)
  onHead: () => void;
  onFold: () => void;
  onViewed: () => void;
  children?: ReactNode; // its lines, when they show
}) {
  return (
    <section id={`file-${index}`} className={cx(css.file, viewed && css.viewed, folded && !viewed && css.folded)}>
      <div id={`row-${index}`} className={cx(css.head, cursor && css.cursor)} onClick={onHead}>
        <span className={cx(css.status, statusClass[status])}>{capital(status)}</span>
        <Text mono>
          {oldPath && <Text dim>{oldPath} → </Text>}
          {path}
        </Text>
        <span className={css.counts}>
          {additions > 0 && <span className={css.plus}>+{additions}</span>}{" "}
          {deletions > 0 && <span className={css.minus}>−{deletions}</span>}
        </span>
        {unchanged && (
          <Button
            variant="ghost"
            size="small"
            className={css.unchanged}
            onClick={(e) => (e.stopPropagation(), onFold())}
            title="Its change is the same as when you reviewed it: a rebase moved it, and nothing else"
          >
            <Icon name="check" size={11} /> Unchanged since you reviewed · {folded ? "show" : "hide"}
          </Button>
        )}
        {skim && !unchanged && (
          <Button variant="ghost" size="small" className={css.skim} onClick={(e) => (e.stopPropagation(), onFold())} title="It's one to skim, so it's folded: it still counts until you mark it viewed">
            {skim} · {folded ? "show" : "hide"}
          </Button>
        )}
        <Checkbox className={css.viewedBox} checked={viewed} onChange={onViewed} {...keyProps("x", viewed ? "Mark it not viewed" : "Mark it viewed", (e: React.MouseEvent<HTMLLabelElement>) => e.stopPropagation())}>
          Viewed
        </Checkbox>
      </div>
      {guide && !viewed && (
        <p className={css.guide} title="What the guide says the change does to it">
          <Icon name="sparkle" size={13} className={css.guideIcon} />
          <span>
            <Inline text={guide} />
          </span>
        </p>
      )}
      {children}
    </section>
  );
}
