import { cx } from "./cx";
import css from "./Rows.module.css";

// Rows is a list of rows, each a <Row>.
export function Rows({ className, ...rest }: React.ComponentProps<"ul">) {
  return <ul className={cx(css.rows, className)} {...rest} />;
}

type RowProps = React.ComponentProps<"li"> & {
  // selected: the row the keys are on (shown only while they move it).
  selected?: boolean;
  // picked: the row chosen for something, shown whatever moves.
  picked?: boolean;
  // flash: just arrived or changed, it lights up once.
  flash?: boolean;
  // still: a row with no link in it, laid out as a link would be.
  still?: boolean;
};

// Row is a row of a list. Its link (a <Link>, an <a>) takes rowLinkClass.
export function Row({ selected, picked, flash, still, className, ...rest }: RowProps) {
  // data-cursor: the row the keys are on, for what acts on it from outside
  // the list (z peeks at what it goes to: chrome/usePeekKey.ts).
  return <li className={cx(css.row, selected && css.selected, picked && css.picked, flash && css.flash, still && css.still, className)} data-cursor={selected ? "" : undefined} {...rest} />;
}

// rowLinkClass is a row's link's look: the whole row, laid out.
export function rowLinkClass(className?: string): string {
  return cx(css.link, className);
}

// rowStatusClass places a row's status icon at its first line.
export function rowStatusClass(className?: string): string {
  return cx(css.status, className);
}

// as: a div, where what's in it is laid out in blocks of its own.
type PartProps = React.HTMLAttributes<HTMLElement> & { as?: "span" | "div" };

function part(own: string) {
  return function Part({ as: Tag = "span", className, ...rest }: PartProps) {
    return <Tag className={cx(own, className)} {...rest} />;
  };
}

// RowMain is the row's main part, taking what's left of its width: its
// RowLines, a RowSub under them.
export const RowMain = part(css.main);
// RowLine is a line of it: a RowTitle and what's beside it.
export const RowLine = part(css.line);
// RowTitle is the row's name, cut short with an ellipsis.
export const RowTitle = part(css.title);
// RowSub is the quieter line under it.
export const RowSub = part(css.sub);
// RowMeta is a fact at the row's end, gone on a narrow screen.
export const RowMeta = part(css.meta);
