import type { ReactNode } from "react";
import type { CalloutTone } from "../../ui/Callout";
import { cx } from "../../ui/cx";
import type { IconName } from "../../ui/Icon";
import { StatusIcon } from "../../ui/StatusIcon";
import { toneClass } from "../../ui/tone";
import css from "./MissionLead.module.css";

// MissionLead is the first of a mission's cards, as its Summary's subject
// rather than a notice on it: the step the mission's on over what's
// happening, at a title's size; what moves it on beside that when it's one
// thing to do, or under it (wide) when it's choices and an answer, a
// field, a question to answer; and crew's last word (latest), so "what
// now, and what's mine" is one block. Its tone is a wash, not an edge.
export function MissionLead({
  tone,
  icon,
  live,
  title,
  step,
  actions,
  wide,
  latest,
  children,
}: {
  tone: CalloutTone;
  icon: IconName;
  live?: boolean;
  title: ReactNode;
  step?: ReactNode;
  actions?: ReactNode;
  wide?: boolean;
  latest?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className={cx(css.lead, toneClass(tone))} role={tone === "danger" ? "alert" : undefined} aria-label="Status">
      <div className={css.main}>
        <div className={css.body}>
          {step && <div className={css.step}>{step}</div>}
          <div className={css.line}>
            <StatusIcon icon={icon} tone={tone} live={live} size={20} className={css.icon} />
            <h2 className={css.title}>{title}</h2>
          </div>
          <div className={css.text}>{children}</div>
        </div>
        {actions && <div className={cx(css.actions, wide && css.wide)}>{actions}</div>}
      </div>
      {latest}
    </section>
  );
}
