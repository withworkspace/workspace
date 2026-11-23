import { type ReactNode, type Ref, useState } from "react";
import { Button } from "./Button";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { Input } from "./Input";
import css from "./PageBar.module.css";
import { TLink } from "./TLink";

type PageBarProps = {
  // The way back: where to, what it says when pointed at, and its name
  // for a screen reader where that says more ("Back to the missions").
  back: { to: string; title: string; label?: string };
  // What it's doing, before its name (a Status, icon only).
  status?: ReactNode;
  // Its name: the page's h1, cut short before it pushes the rest off.
  // Left out while the page loads, it's a blank the size of one.
  title?: string;
  // Renaming it in place: a pencil beside the name on hover, which makes
  // the name a text box. Called with the new name, when it's another.
  onRename?: (to: string) => void;
  // What the pencil says to a screen reader ("Rename the mission").
  renameLabel?: string;
  // Beside its name: its pin, what it's about. It gives way first.
  children?: ReactNode;
  // At the bar's end, what's done to it: in the row.
  actions?: ReactNode;
  // At the bar's end, the views of it: Tabs (with pageBarTabsClass), on
  // the bar's rule.
  tabs?: ReactNode;
  // What hangs from the bar, under its rule and over the page: a card
  // that slides in from its edge.
  under?: ReactNode;
  // The page's own class, to place it.
  className?: string;
  ref?: Ref<HTMLDivElement>;
};

// PageBar is the bar over one thing's page (a mission, a conversation):
// back, what it's doing, its name, and at its end what's done to it or
// the views of it. It's one row over a rule, which stays at the top
// while the page scrolls under it.
export function PageBar({ back, status, title, onRename, renameLabel = "Rename", children, actions, tabs, under, className, ref }: PageBarProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(title ?? "");
  const save = () => {
    const to = value.trim();
    setEditing(false);
    if (to && to !== title) onRename?.(to);
  };
  return (
    <div ref={ref} className={cx(css.bar, className)} data-page-bar="">
      <div className={css.row}>
        <header className={css.head}>
          <TLink to={back.to} className={css.back} title={back.title} aria-label={back.label ?? back.title}>
            <Icon name="arrow-left" size={15} />
          </TLink>
          {status && <span className={css.status}>{status}</span>}
          {title == null ? (
            <h1 className={css.blank}>&nbsp;</h1>
          ) : editing ? (
            <Input
              className={css.titleInput}
              autoFocus
              value={value}
              maxLength={50}
              aria-label={renameLabel}
              onChange={(e) => setValue(e.target.value)}
              onBlur={save}
              onKeyDown={(e) => {
                if (e.key === "Enter") save();
                if (e.key === "Escape") {
                  e.preventDefault();
                  setEditing(false);
                }
              }}
            />
          ) : (
            <h1 className={css.title}>
              <span className={css.titleText} title={title}>
                {title}
              </span>
              {onRename && (
                <Button
                  variant="ghost"
                  size="small"
                  square
                  className={css.rename}
                  onClick={() => {
                    setValue(title);
                    setEditing(true);
                  }}
                  title="Rename"
                  aria-label={renameLabel}
                >
                  <Icon name="pencil" size={12} />
                </Button>
              )}
            </h1>
          )}
          {children}
        </header>
        {actions && <div className={css.actions}>{actions}</div>}
        {tabs}
      </div>
      {under}
    </div>
  );
}

// pageBarTabsClass is for the Tabs at a PageBar's end: they sit on the
// bar's rule, with none of their own.
export function pageBarTabsClass(className?: string): string {
  return cx(css.tabs, className);
}
