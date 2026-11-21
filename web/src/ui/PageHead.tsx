import css from "./PageHead.module.css";
import { cx } from "./cx";

type PageHeadProps = {
  title: React.ReactNode;
  // actions are the page's own buttons, at the head's right; left out
  // when there are none (false, null).
  actions?: React.ReactNode;
  // children are beside the title: a state, a count.
  children?: React.ReactNode;
  // lead is before the title: the way back to where the page is in.
  lead?: React.ReactNode;
  className?: string;
};

// PageHead is the head of a page: its title (the page's h1), what's beside
// it, and its actions at the right.
export function PageHead({ title, actions, children, lead, className }: PageHeadProps) {
  return (
    <header className={cx(css.head, className)}>
      {lead}
      <h1>{title}</h1>
      {children}
      {actions ? <div className={css.actions}>{actions}</div> : null}
    </header>
  );
}
