import type { MouseEvent } from "react";
import { Link } from "react-router";
import { GroupHead } from "../../ui/GroupHead";
import { Icon } from "../../ui/Icon";
import { Row, RowMain, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../../ui/Rows";
import css from "./ReaderCiteList.module.css";
import { Text, textClass } from "../../ui/Text";

export interface CiteItem {
  key: string;
  to: string;
  title: string;
  // Where: its file's name and the line.
  file: string;
  line: number;
  // By a link in it, or naming it.
  how: "link" | "name" | (string & {});
  onContextMenu?: (e: MouseEvent) => void;
}

// ReaderCiteList is documents citing this one, or cited by it: each by its
// title, and where (by a link in it, or naming it).
export function ReaderCiteList({ title, items }: { title: string; items: CiteItem[] }) {
  return (
    <section className={css.list}>
      <GroupHead count={items.length}>{title}</GroupHead>
      <Rows>
        {items.map((c) => (
          <Row key={c.key}>
            <Link to={c.to} className={rowLinkClass()} onContextMenu={c.onContextMenu}>
              <Icon name="book" size={14} className={rowStatusClass(textClass("dim"))} />
              <RowMain>
                <RowTitle>{c.title}</RowTitle>
              </RowMain>
              <Text dim mono className={css.at} title={c.how === "link" ? "By link" : "By name"}>
                {c.file}:{c.line}
              </Text>
            </Link>
          </Row>
        ))}
      </Rows>
    </section>
  );
}
