import type { MouseEvent, ReactNode } from "react";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Inline } from "../../ui/Inline";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../../ui/Rows";
import { Status, type StatusInfo } from "../../ui/Status";
import { TLink } from "../../ui/TLink";
import { Time } from "../../ui/Time";
import css from "./MissionListRow.module.css";

type MissionListRowProps = {
  // The row's id, for scrolling to it (mission-<slug>).
  id: string;
  to: string;
  status: StatusInfo;
  title: string;
  // Its crew at work, as MissionListCrew chips.
  crew?: ReactNode;
  // Where it stands, in a sentence under its title (title: the whole of
  // it, on hover): what ws is running while it checks a step, or what its
  // headline says.
  sub?: { title: string; checking?: ReactNode; said: string };
  // Where it is on its route, in a few words (title: the whole sentence).
  route?: { words: string; title: string };
  // Its capsules, named (title: each of them).
  capsules?: { words: string; title: string };
  // When it last moved: a time ago, or (past) when.
  time: { iso: string; as: "age" | "stamp" };
  selected?: boolean;
  flash?: boolean;
  onPoint?: () => void;
  onOpen?: () => void;
  onContextMenu?: (e: MouseEvent) => void;
};

// MissionListRow is a mission in the missions list: its status, its title
// (its crew at work beside it) with where it stands under it, then a chip
// of its status's words, where it is on its route, its capsules and how
// long since it moved. On a narrow window what's after the title wraps
// under it.
export function MissionListRow({ id, to, status, title, crew, sub, route, capsules, time, selected, flash, onPoint, onOpen, onContextMenu }: MissionListRowProps) {
  return (
    <Row selected={selected} flash={flash} id={id} role="option" aria-selected={selected} className={css.row} onMouseMove={onPoint}>
      <TLink to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu} onClick={onOpen}>
        <Status s={status} iconOnly size={16} className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            <RowTitle>{title}</RowTitle>
            {crew}
          </RowLine>
          {sub && <RowSub title={sub.title}>{sub.checking ?? <Inline text={sub.said} lines={1} className={css.said} />}</RowSub>}
        </RowMain>
        <Chip tone={status.tone} className={css.chip}>
          {status.words}
        </Chip>
        {route && <RowMeta title={route.title}>{route.words}</RowMeta>}
        {capsules && (
          <RowMeta title={capsules.title}>
            <Icon name="box" size={13} />
            {capsules.words}
          </RowMeta>
        )}
        <Time iso={time.iso} as={time.as} className={cx("age", css.age)} />
      </TLink>
    </Row>
  );
}
