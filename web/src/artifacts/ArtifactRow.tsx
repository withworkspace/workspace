import type { MouseEventHandler, ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Row, RowLine, RowMain, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import { cx } from "../ui/cx";
import css from "./ArtifactRow.module.css";

type ArtifactRowProps = {
  // Where it goes, and what the other button of the mouse opens on it.
  to: string;
  onContextMenu?: MouseEventHandler;
  // In a list the keys move through: its id there, whether the keys are
  // on it, and what the pointer moving over it does.
  id?: string;
  option?: boolean;
  selected?: boolean;
  onHover?: () => void;
  // Where it stands, as its mark: the artifact's icon, with none.
  status?: StatusInfo;
  title: string;
  // dimmed is one that's off the list or about to be: its title's quiet.
  dimmed?: boolean;
  // Beside its title: its version, where it stands, when it expires.
  chips?: ReactNode;
  // Its latest version's note, on a line.
  note?: string;
  // What's said under it, each in a span: where it came from, where it's
  // kept, its ref.
  children: ReactNode;
  // At its end: who by and when, with a title that says more.
  when?: { by: string; at: string; title: string };
  // Promote, beside it: for one in its last week, or expired.
  onPromote?: () => void;
};

// ArtifactRow is an artifact in a list: this machine's, or one found in a
// repo.
export function ArtifactRow({ to, onContextMenu, id, option, selected, onHover, status, title, dimmed, chips, note, children, when, onPromote }: ArtifactRowProps) {
  return (
    <Row selected={selected} id={id} role={option ? "option" : undefined} aria-selected={option ? !!selected : undefined} className={cx(css.row, dimmed && css.dimmed)} onMouseMove={onHover}>
      <TLink to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu}>
        {status ? <Status s={status} iconOnly size={16} className={rowStatusClass()} /> : <Icon name="artifact" size={16} className={rowStatusClass()} />}
        <RowMain>
          <RowLine>
            <RowTitle className={css.title}>{title}</RowTitle>
            {chips}
          </RowLine>
          {note && (
            <RowSub>
              <Inline text={note} lines={1} />
            </RowSub>
          )}
          <RowSub className={css.meta}>{children}</RowSub>
        </RowMain>
        {when && (
          <span className={css.when} title={when.title}>
            {when.by}
            {when.by && " · "}
            <Time iso={when.at} as="age" />
          </span>
        )}
      </TLink>
      {onPromote && (
        <Button variant="ghost" size="small" className={css.promote} onClick={onPromote} title="Keep it for good: it expires no more">
          <Icon name="check-circle" size={13} /> Promote
        </Button>
      )}
    </Row>
  );
}

// artifactRowCodeClass is an id or a path in the row's last line (as Text
// mono): smaller and fainter than the words beside it.
export const artifactRowCodeClass: string = css.code;
