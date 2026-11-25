import type { MouseEvent } from "react";
import { Link } from "react-router";
import { GroupHead } from "../../ui/GroupHead";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../../ui/Rows";
import { Status, type StatusInfo } from "../../ui/Status";
import { Time } from "../../ui/Time";
import css from "./ReaderTalks.module.css";
import { textClass } from "../../ui/Text";

export interface TalkItem {
  id: string;
  to: string;
  title: string;
  summary?: string;
  status: StatusInfo;
  // The passage it was asked of, and its section.
  sub?: string;
  // When it last changed.
  at: string;
  onContextMenu?: (e: MouseEvent) => void;
}

// ReaderTalks is the conversations about a document: asks that were kept,
// each by its name, with the passage it was asked of, and when it last
// changed.
export function ReaderTalks({ items }: { items: TalkItem[] }) {
  return (
    <section>
      <GroupHead count={items.length}>Conversations about it</GroupHead>
      <Rows>
        {items.map((t) => (
          <Row key={t.id}>
            <Link to={t.to} className={rowLinkClass()} title={t.summary} onContextMenu={t.onContextMenu}>
              <Status s={t.status} iconOnly size={14} className={rowStatusClass()} />
              <RowMain>
                <RowLine>
                  <RowTitle>{t.title}</RowTitle>
                </RowLine>
                {t.sub && <RowSub className={textClass("dim")}>{t.sub}</RowSub>}
              </RowMain>
              <Time iso={t.at} as="age" className={css.age} />
            </Link>
          </Row>
        ))}
      </Rows>
    </section>
  );
}
