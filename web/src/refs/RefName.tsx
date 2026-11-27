import type { CSSProperties, ReactNode } from "react";
import { Link } from "react-router";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import type { PageRef } from "./refs";
import css from "./RefName.module.css";

const kinds: Partial<Record<PageRef["kind"], string>> = { mission: css.mission, page: css.page, talk: css.talk, pr: css.pr, issue: css.pr, thing: css.thing };

type RefNameProps = {
  kind: PageRef["kind"];
  // words: marked as somewhere to go; pill: in a pill of its accent.
  shape: "words" | "pill";
  // words: the writer's own words, not the thing's name: its kind's icon
  // goes before them.
  words?: boolean;
  // icon is shown before it (words of the writer's own, a pill).
  icon?: IconName;
  // accent is a CSS colour for a pill (a repo's).
  accent?: string;
  // Where it goes: to, in ws; href, outside, in a tab of its own; or, with
  // neither, words that go there when clicked (onClick), in a line that
  // may sit inside a link already.
  to?: string;
  href?: string;
  onClick?: (e: React.MouseEvent<HTMLElement>) => void;
  title?: string;
  dataRef: string;
  onMouseEnter?: (e: React.MouseEvent<HTMLElement>) => void;
  onMouseLeave?: () => void;
  onFocus?: (e: React.FocusEvent<HTMLElement>) => void;
  onBlur?: () => void;
  children?: ReactNode;
};

// RefName is a name in what's read, drawn as somewhere to go: its words,
// marked, or a pill. It has its look only in markdown, and in words that
// say data-refs (Inline, a list of refs): elsewhere it's a plain link.
export function RefName({ kind, shape, words, icon, accent, to, href, onClick, title, dataRef, onMouseEnter, onMouseLeave, onFocus, onBlur, children }: RefNameProps) {
  const props = {
    className: cx(css.name, kinds[kind], shape === "pill" && css.pill, words && css.words),
    style: accent ? ({ "--ref-accent": accent } as CSSProperties) : undefined,
    title,
    "data-ref": dataRef,
    onMouseEnter,
    onMouseLeave,
    onFocus,
    onBlur,
  };
  const inside = (
    <>
      {icon && <Icon name={icon} size={12} className={css.icon} />}
      {children}
    </>
  );
  if (to !== undefined)
    return (
      <Link to={to} {...props}>
        {inside}
      </Link>
    );
  if (href !== undefined)
    return (
      <a href={href} target="_blank" rel="noreferrer" {...props}>
        {inside}
      </a>
    );
  return (
    <span {...props} role="link" tabIndex={0} onClick={onClick}>
      {inside}
    </span>
  );
}
