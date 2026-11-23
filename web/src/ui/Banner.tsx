import type { ReactNode } from "react";
import css from "./Banner.module.css";
import { cx } from "./cx";
import { Icon, type IconName } from "./Icon";
import { type Tone, toneClass } from "./tone";

// Banner is a line across the top of what it's about: something went
// wrong (danger, unless a tone says otherwise), something changed under
// you, something was done. Its words take the room, after an icon where
// there's one, and what's pressed (actions) stays at its end. In danger
// it's an alert to a screen reader, unless a role says otherwise. A
// Callout is for more than a line: a title, a paragraph.
export function Banner({
  tone = "danger",
  icon,
  children,
  actions,
  role,
  className,
}: {
  tone?: Tone;
  icon?: IconName;
  children: ReactNode;
  actions?: ReactNode;
  role?: "alert" | "status";
  className?: string;
}) {
  return (
    <div className={cx(css.banner, toneClass(tone), className)} role={role ?? (tone === "danger" ? "alert" : undefined)}>
      {icon && <Icon name={icon} size={13} />}
      <span className={css.words}>{children}</span>
      {actions}
    </div>
  );
}
