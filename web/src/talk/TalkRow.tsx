import type { MouseEventHandler } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Row, RowLine, RowMain, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import css from "./TalkRow.module.css";

// TalkRow is a conversation in the list: how it stands, its name, what's
// asked of you in it (your turn, proposals to confirm), what it's about
// (a pull request, a document) or became (a mission), its summary in a
// line, and when it last moved. selected is the keys' cursor on it.
export function TalkRow({
  href,
  id,
  selected,
  status,
  title,
  yourTurn,
  proposed,
  pr,
  doc,
  became,
  summary,
  at,
  onPoint,
  onContextMenu,
}: {
  href: string;
  id?: string;
  selected?: boolean;
  status: StatusInfo;
  title: string;
  yourTurn?: boolean;
  proposed?: number;
  pr?: string;
  doc?: { name: string; title: string };
  became?: string;
  summary?: string;
  at: string;
  onPoint?: () => void;
  onContextMenu?: MouseEventHandler;
}) {
  return (
    <Row selected={selected} id={id} role="option" aria-selected={!!selected} onMouseMove={onPoint}>
      <TLink to={href} className={rowLinkClass(css.link)} onContextMenu={onContextMenu}>
        <Status s={status} iconOnly size={16} className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            <RowTitle>{title}</RowTitle>
            {yourTurn && <Chip tone="attention">Your turn</Chip>}
            {!!proposed && <Chip tone="attention">{proposed} to confirm</Chip>}
            {pr && (
              <Chip tone="quiet" title="The pull request it's about">
                <Icon name="pr" size={11} />
                {pr}
              </Chip>
            )}
            {doc && (
              <Chip tone="quiet" title={doc.title}>
                <Icon name="book" size={11} />
                {doc.name}
              </Chip>
            )}
            {became && (
              <Chip tone="quiet" title="The mission it became">
                <Icon name="flag" size={11} />
                {became}
              </Chip>
            )}
          </RowLine>
          {summary && (
            <RowSub>
              <Inline text={summary} lines={1} />
            </RowSub>
          )}
        </RowMain>
        <Time iso={at} as="age" className="age" />
      </TLink>
    </Row>
  );
}
