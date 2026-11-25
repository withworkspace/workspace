import { Link } from "react-router";
import { cx } from "../ui/cx";
import { Icon, type IconName } from "../ui/Icon";
import css from "./TalkSideItem.module.css";

// TalkSideItem is a thing beside a conversation, its kind's icon, its
// name and its tag (a ref): a link to it (to), or a button that goes to
// its card (onClick). open: it's waiting to be confirmed. grow: it takes
// the line's room, beside what else is on it (TalkSideMade).
export function TalkSideItem({ icon, name, tag, to, onClick, open, grow }: { icon: IconName; name: string; tag?: string; to?: string; onClick?: () => void; open?: boolean; grow?: boolean }) {
  const className = cx(css.item, open && css.open, grow && css.grow);
  const inside = (
    <>
      <Icon name={icon} size={12} className={css.icon} />
      <span>
        {name}
        {tag && <span className={css.tag}> {tag}</span>}
      </span>
    </>
  );
  if (to)
    return (
      <Link to={to} className={className}>
        {inside}
      </Link>
    );
  return (
    <button className={className} onClick={onClick}>
      {inside}
    </button>
  );
}
