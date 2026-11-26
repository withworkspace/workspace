import type { ReactNode } from "react";
import { Copyable } from "../../ui/Copyable";
import { Icon, type IconName } from "../../ui/Icon";
import css from "./MissionWorkHead.module.css";

type MissionWorkHeadProps = {
  // A capsule's: its icon, its repo's name, and the capsule to copy.
  icon?: IconName;
  name?: ReactNode;
  slug?: string;
  // An output's: what kind of thing it is and when it was made, in small
  // words, with its title (title) on a line of its own under.
  kind?: ReactNode;
  title?: ReactNode;
  // After its name: its branch, where it stands.
  tags?: ReactNode;
  // What can be done with it, at the top line's end.
  actions?: ReactNode;
  // What's known of it, in a quiet line that wraps.
  meta?: ReactNode;
  // Its pull request's title.
  pr?: ReactNode;
};

// MissionWorkHead is the head of what's open in the Work tab: what it is
// and where it stands, what can be done with it, and what's known of it.
// It stays while what's under it scrolls.
export function MissionWorkHead({ icon, name, slug, kind, title, tags, actions, meta, pr }: MissionWorkHeadProps) {
  return (
    <header className={css.head}>
      <div className={css.top}>
        {icon && <Icon name={icon} size={16} />}
        {name && <h2>{name}</h2>}
        {slug && <Copyable text={slug} className={css.slug} title="Copy capsule" />}
        {kind && <span className={css.kind}>{kind}</span>}
        {tags}
        <div className={css.actions}>{actions}</div>
      </div>
      {title && <h2 className={css.title}>{title}</h2>}
      <div className={css.meta}>{meta}</div>
      {pr !== undefined && <div className={css.pr}>{pr}</div>}
    </header>
  );
}
