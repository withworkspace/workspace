import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./TeamEvents.module.css";
import { Text, textClass } from "../ui/Text";

// TeamEvents is what's under a change on the Team page while it's open:
// its pull requests, where it's in several repos (TeamPulls), what happened
// to it (TeamEventLine, each a line), why what it does couldn't be written,
// and what can be done with it (actions).
export function TeamEvents({ pulls, failed, actions, children }: { pulls?: ReactNode; failed?: ReactNode; actions?: ReactNode; children: ReactNode }) {
  return (
    <div className={css.events}>
      {pulls}
      <ol className={css.list}>{children}</ol>
      {failed && <p className={css.failed}>{failed}</p>}
      <div className={css.actions}>{actions}</div>
    </div>
  );
}

type TeamEventLineProps = {
  at: string;
  icon: IconName;
  // Who did what: its words (TeamEventSubject for what landed).
  children: ReactNode;
  // The commit it was, if it was one.
  commit?: string;
  // The files it changed: the first few, and how many more.
  paths?: string[];
  files?: number;
};

// TeamEventLine is one thing that happened to a change: when, what (an icon,
// who did it and what), its commit, the files it changed.
export function TeamEventLine({ at, icon, children, commit, paths, files }: TeamEventLineProps) {
  const more = (files ?? paths?.length ?? 0) - 3;
  return (
    <li className={css.event}>
      <Time iso={at} as="clock" className={cx(textClass("dim"), css.clock)} />
      <Icon name={icon} size={13} className={css.icon} />
      <span className={css.words}>{children}</span>
      {commit && <Text mono dim>{commit.slice(0, 7)}</Text>}
      {!!paths?.length && (
        <Text dim className={css.paths} title={paths.join("\n")}>
          {paths.slice(0, 3).join(", ")}
          {more > 0 ? `, and ${more} more` : ""}
        </Text>
      )}
    </li>
  );
}

// TeamEventSubject is what a commit that landed says it does.
export function TeamEventSubject({ children }: { children: ReactNode }) {
  return <span className={css.subject}>{children}</span>;
}
