import type { ReactNode } from "react";
import css from "./Callout.module.css";
import { cx } from "./cx";
import { Icon, type IconName } from "./Icon";
import { toneClass } from "./tone";

export type CalloutTone = "attention" | "info" | "danger" | "done" | "quiet" | "progress";

const toneIcon: Record<CalloutTone, IconName> = {
  attention: "alert",
  info: "info",
  danger: "x-circle",
  done: "check-circle",
  quiet: "info",
  progress: "circle-dot",
};

// Callout is a note in the flow of a page: a tinted panel with a bar on
// its left, an icon (its tone's, one named, or with null none), a title,
// what follows and what's pressed. big is one that leads what's under
// it: its title a size up. Where it sits, and the room under it, are the
// page's, by the class it passes.
export function Callout({
  tone = "info",
  icon,
  title,
  children,
  actions,
  big,
  className,
}: {
  tone?: CalloutTone;
  icon?: IconName | null;
  title?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  big?: boolean;
  className?: string;
}) {
  const ic = icon === null ? undefined : (icon ?? toneIcon[tone]);
  return (
    <div className={cx(css.callout, tone === "quiet" && css.quiet, big && css.big, toneClass(tone), className)} role={tone === "danger" ? "alert" : undefined} data-callout="">
      {ic && <Icon name={ic} size={16} className={css.icon} />}
      <div className={css.body}>
        {title && <div className={css.title}>{title}</div>}
        {children && <div className={css.text}>{children}</div>}
        {actions && <div className={css.actions}>{actions}</div>}
      </div>
    </div>
  );
}
