import type { MouseEventHandler, ReactNode } from "react";
import { Link } from "react-router";
import type { PR } from "../api";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { PullRequestCheckMark } from "./PullRequestCheckMark";
import css from "./ReviewsRow.module.css";
import { cx } from "../ui/cx";
import { Text } from "../ui/Text";

// ReviewsRow is a pull request, or a stack of them, on the Reviews page:
// where it stands with you, its title and chips, who and where it's from,
// how much it changes, its checks and how long since it moved. It opens
// here (to) or on GitHub (href, onOpen). A stack folds its layers (l),
// which are under it. act is what's done to it from the row (merging it):
// it's shown in place of its size, checks and age while the row's pointed
// at or picked, so the columns stay where every other row has them.
export function ReviewsRow({
  id,
  selected,
  status,
  title,
  chips,
  sub,
  additions,
  deletions,
  files,
  checks,
  updated,
  to,
  href,
  onHover,
  onOpen,
  onContextMenu,
  fold,
  act,
  layers,
}: {
  id: string;
  selected: boolean;
  status: StatusInfo;
  title: string;
  chips?: ReactNode;
  sub: ReactNode;
  additions: number;
  deletions: number;
  files?: number;
  checks?: PR["checks"];
  updated?: string;
  to?: string;
  href?: string;
  onHover: () => void;
  onOpen: () => void; // a link here, as it's followed; one to GitHub, instead
  onContextMenu?: MouseEventHandler;
  fold?: { folded: boolean; onFold: () => void };
  act?: ReactNode;
  layers?: ReactNode;
}) {
  const body = (
    <>
      <Status s={status} iconOnly size={16} className={rowStatusClass()} />
      <RowMain>
        <RowLine>
          <RowTitle>{title}</RowTitle>
          {chips}
        </RowLine>
        <RowSub>{sub}</RowSub>
      </RowMain>
      <span className={css.end}>
        <RowMeta className={css.size} title={files !== undefined ? `${files} ${files === 1 ? "file" : "files"} changed` : undefined}>
          <span className={css.plus}>+{additions}</span> <span className={css.minus}>−{deletions}</span>
          {files !== undefined && (
            <Text dim className={css.files}>
              {files} {files === 1 ? "file" : "files"}
            </Text>
          )}
        </RowMeta>
        <PullRequestCheckMark checks={checks} />
        {updated ? <Time iso={updated} as="age" className="age" /> : <span className="age" />}
      </span>
    </>
  );
  return (
    <Row selected={selected} id={id} role="option" aria-selected={selected} className={cx(css.row, !!act && css.withAct, selected && css.picked)} onMouseMove={onHover}>
      {to ? (
        <Link to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu} onClick={onOpen}>
          {body}
        </Link>
      ) : (
        <a
          href={href}
          target="_blank"
          rel="noreferrer"
          className={rowLinkClass(css.link)}
          title="Merged: opens on GitHub"
          onClick={(e) => {
            e.preventDefault();
            onOpen();
          }}
        >
          {body}
        </a>
      )}
      {act && <span className={css.act}>{act}</span>}
      {fold && (
        <Button variant="ghost" size="small" square className={css.fold} onClick={fold.onFold} title={`${fold.folded ? "Show" : "Hide"} layers`} keys="l" aria-expanded={!fold.folded} aria-label="Layers">
          <Icon name={fold.folded ? "chevron-right" : "chevron-down"} size={13} />
        </Button>
      )}
      {layers}
    </Row>
  );
}
