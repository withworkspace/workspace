import type { MouseEventHandler, ReactNode } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Row, RowLine, RowMain, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import css from "./PagesRow.module.css";
import { Text, textClass } from "../ui/Text";

type PagesRowProps = {
  id: string;
  title: string;
  to: string;
  // Its kind of document: its name, and what it is.
  kind?: { title: string; hint: string };
  // The file it's tied to: its name, and its ref.
  file?: { name: string; ref: string };
  filling?: number;
  cells?: number;
  stale?: number;
  updated: string;
  // More of it, after its chips (where it's filed, say).
  note?: ReactNode;
  selected: boolean;
  onHover: () => void;
  onContextMenu?: MouseEventHandler;
};

// PagesRow is a page in a list: its title, its kind, the file it's tied
// to, its cells (filling, or out of date), its name and when it last
// changed.
export function PagesRow({ id, title, to, kind, file, filling, cells, stale, updated, note, selected, onHover, onContextMenu }: PagesRowProps) {
  return (
    <Row selected={selected} id={`page-${id}`} role="option" aria-selected={selected} onMouseMove={onHover}>
      <TLink to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu}>
        <Icon name="file" size={16} className={rowStatusClass(css.status)} />
        <RowMain>
          <RowLine>
            <RowTitle>{title}</RowTitle>
            {kind && (
              <Chip tone="quiet" title={kind.hint}>
                {kind.title}
              </Chip>
            )}
            {file && (
              <Chip tone="quiet" className={textClass("mono")} title={`Tied to ${file.ref}: saving writes there`}>
                <Icon name="file" size={11} /> {file.name}
              </Chip>
            )}
            {!!filling && (
              <Chip tone="progress">
                <Spinner size={11} /> Filling {filling === 1 ? "a cell" : `${filling} cells`}
              </Chip>
            )}
            {!!cells && !filling && <Chip tone="quiet">{cells === 1 ? "1 cell" : `${cells} cells`}</Chip>}
            {!!stale && !filling && (
              <Chip tone="attention" title="Sources changed since written">
                {stale === 1 ? "1 out of date" : `${stale} out of date`}
              </Chip>
            )}
            {note}
          </RowLine>
        </RowMain>
        <Text dim className={css.ref}>{id}</Text>
        <Time iso={updated} as="age" className="age" />
      </TLink>
    </Row>
  );
}
