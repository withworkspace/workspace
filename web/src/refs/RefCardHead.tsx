import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { HoverCardHead, HoverCardTitle } from "../ui/HoverCard";
import { Icon, type IconName } from "../ui/Icon";
import css from "./RefCardHead.module.css";

type RefCardHeadProps = {
  // What kind of thing it is, as an icon.
  icon: IconName;
  title: ReactNode;
  // mono: its name is a path, in the code's type.
  mono?: boolean;
  // A quieter name after it (a repo's own, after its label).
  aside?: string;
};

// RefCardHead is the head of a ref's card: its kind's icon and its name.
export function RefCardHead({ icon, title, mono, aside }: RefCardHeadProps) {
  return (
    <HoverCardHead className={css.head}>
      <Icon name={icon} size={13} className="dim" />
      <HoverCardTitle className={mono ? cx("mono", css.mono) : undefined}>{title}</HoverCardTitle>
      {aside && <span className={cx("dim mono", css.mono)}>{aside}</span>}
    </HoverCardHead>
  );
}
