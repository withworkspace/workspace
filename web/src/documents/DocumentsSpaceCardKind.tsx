import { cx } from "../ui/cx";
import { Count } from "../ui/Count";
import { Time } from "../ui/Time";
import { TLink } from "../ui/TLink";
import css from "./DocumentsSpaceCardKind.module.css";
import { textClass } from "../ui/Text";

type DocumentsSpaceCardKindProps = {
  to: string; // its list on the space's page
  name: string;
  folder?: boolean; // it's a folder: its name's a path
  count: number;
  last?: { title: string; at: string }; // which changed last, and when
  stands?: string; // how many stand where: "2 proposed · 1 accepted"
};

// DocumentsSpaceCardKind is a kind of document on a space's card (or a
// folder, where it has no kinds): how many, which changed last, and where
// they stand, to its list.
export function DocumentsSpaceCardKind({ to, name, folder, count, last, stands }: DocumentsSpaceCardKindProps) {
  return (
    <li>
      <TLink to={to} className={css.link}>
        <span className={cx(css.kind, folder && css.folder, folder && textClass("mono"))}>{name}</span> <Count>{count}</Count>
        {last && (
          <span className={css.last} title={`Changed last: ${last.title}`}>
            {last.title} · <Time iso={last.at} as="age" />
          </span>
        )}
        {stands && <span className={css.stands}>{stands}</span>}
      </TLink>
    </li>
  );
}
