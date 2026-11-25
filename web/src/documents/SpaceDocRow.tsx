import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Row, RowLine, RowMain, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import css from "./SpaceDocRow.module.css";

type SpaceDocRowProps = {
  id: string;
  to: string; // the document, in the reader
  title: string;
  selected: boolean; // the keys' cursor is on it
  picked?: boolean; // it's shown beside the list
  onHover?: () => void; // the cursor follows the pointer
  onClick?: (e: React.MouseEvent) => void; // picks it, where rows are picked
  onDoubleClick?: () => void; // opens it, where rows are picked
  onContextMenu?: (e: React.MouseEvent) => void;
  made?: string; // its number or date, in a column (a kind's list, where any has one)
  chips?: ReactNode; // where it stands, in flight
  hit?: ReactNode; // where a search found it, a SpaceHitLine
  changed?: { by?: string; at: string }; // who changed it last, and when (a kind's list)
  file?: string; // its file's name (a folder's list): a file icon leads it
};

// SpaceDocRow is a document in a space's list: a kind's (its number or
// date, where it stands, who changed it last) or a folder's (its file).
export function SpaceDocRow({ id, to, title, selected, picked, onHover, onClick, onDoubleClick, onContextMenu, made, chips, hit, changed, file }: SpaceDocRowProps) {
  return (
    <Row selected={selected} picked={picked} id={id} role="option" aria-selected={selected} onMouseMove={onHover}>
      <TLink to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu} onClick={onClick} onDoubleClick={onDoubleClick}>
        {file !== undefined && <Icon name="file" size={16} className={rowStatusClass()} />}
        {made !== undefined && <span className={cx(css.made, "mono dim")}>{made}</span>}
        <RowMain>
          <RowLine>
            <RowTitle>{title}</RowTitle>
            {chips}
          </RowLine>
          {hit}
        </RowMain>
        {changed && (
          <span className={cx(css.changed, "dim")} title={`Last changed by ${changed.by ?? "someone"}`}>
            {changed.by?.split(" ")[0]} · <Time iso={changed.at} as="age" />
          </span>
        )}
        {file !== undefined && <span className={cx("dim mono", css.file)}>{file}</span>}
      </TLink>
    </Row>
  );
}
