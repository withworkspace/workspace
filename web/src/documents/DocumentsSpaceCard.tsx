import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { TLink } from "../ui/TLink";
import css from "./DocumentsSpaceCard.module.css";

type DocumentsSpaceCardProps = {
  id: string;
  to: string; // the space's page
  label: string;
  color: string; // its repo's
  folder?: string; // the space's own, when it's a folder of its repo
  repo?: string; // its repo, when its label isn't
  count: string; // how many documents, or that it's not cloned
  selected: boolean; // the keys' cursor is on it
  onHover: () => void;
  children?: ReactNode; // its kinds, a DocumentsSpaceCardKinds
  foot: ReactNode; // a DocumentsSpaceCardFoot
};

// DocumentsSpaceCard is a space on Documents: its name (to its page) and
// how many documents it has, its kinds, and how it stands at its foot.
export function DocumentsSpaceCard({ id, to, label, color, folder, repo, count, selected, onHover, children, foot }: DocumentsSpaceCardProps) {
  return (
    <article id={id} className={cx(css.card, selected && css.selected)} onMouseMove={onHover}>
      <TLink to={to} className={css.head}>
        <span className="repo-dot" style={{ background: color }} />
        <span className={css.title}>{label}</span>
        {folder && <span className="mono dim">{folder}</span>}
        {repo && <span className="mono dim">{repo}</span>}
        <span className={css.count}>{count}</span>
        <Icon name="chevron-right" size={14} className="dim" />
      </TLink>
      {children}
      {foot}
    </article>
  );
}
