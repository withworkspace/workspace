import type { ReactNode } from "react";
import type { SpaceDocChange } from "../api";
import { cx } from "../ui/cx";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import { mark } from "./space";
import css from "./DocumentsChangeLine.module.css";

type DocumentsChangeLineProps = {
  how: SpaceDocChange["how"]; // what was done to the first document it changed
  name: string; // that document's
  path: string;
  to?: string; // to it, unless it's removed
  more?: { count: number; title: string }; // the other documents it changed
  subject?: string; // what the commit said, when it says more than the rest
  space?: ReactNode; // which space, a DocumentsSpaceTag
  by: string; // who
  at: string; // when
};

// DocumentsChangeLine is a commit to documents on one line, by the first
// document it changed: what was done to it, who and when; what the commit
// said, when that says more.
export function DocumentsChangeLine({ how, name, path, to, more, subject, space, by, at }: DocumentsChangeLineProps) {
  return (
    <li className={css.line}>
      <span className={cx(css.how, how === "added" && css.added, how === "removed" && css.removed)} title={how}>
        {mark(how)}
      </span>
      {to ? (
        <TLink to={to} className={css.title} title={path}>
          {name}
        </TLink>
      ) : (
        <span className={cx(css.title, "dim")}>{name}</span>
      )}
      {more && (
        <span className={css.more} title={more.title}>
          +{more.count} more
        </span>
      )}
      <span className={css.what}>{how}</span>
      {subject && (
        <span className={css.subject} title={subject}>
          {subject}
        </span>
      )}
      {space}
      <span className={css.by} title={by}>
        {by.split(" ")[0]} · <Time iso={at} as="age" />
      </span>
    </li>
  );
}
