import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Status, type StatusInfo } from "../ui/Status";
import css from "./RunPanelHead.module.css";
import { Text, textClass } from "../ui/Text";

// RunPanelHead is a run's panel's head: the way back, what it's called
// (title: its recipe's, with its name beside where that's another), which
// run of a task it is (n), where it stands, where it's reached (address:
// opened in a tab of its own, cut short where there's no room), and
// what's done to it (its children).
export function RunPanelHead({
  back,
  title,
  name,
  n,
  status,
  address,
  children,
}: {
  back?: ReactNode;
  title?: string;
  name: string;
  n?: number;
  status: StatusInfo;
  address?: { href: string; host: string; title: string };
  children?: ReactNode;
}) {
  const named = !!title && title !== name;
  return (
    <header className={css.head}>
      {back}
      <h2 className={cx(!named && textClass("mono"), css.name)}>{named ? title : name}</h2>
      {named && <Text mono className={css.n}>{name}</Text>}
      {!!n && <span className={css.n}>run {n}</span>}
      <Status s={status} className={css.status} />
      {address && (
        <a href={address.href} target="_blank" rel="noopener noreferrer" className={cx(textClass("mono"), css.address)} title={address.title}>
          <span className={css.host}>{address.host}</span>
          <Icon name="arrow-up-right" size={11} />
        </a>
      )}
      <span className={css.spacer} />
      <span className={css.actions}>{children}</span>
    </header>
  );
}
