import { Link } from "react-router";
import { chipClass } from "../../ui/Chip";
import { Icon } from "../../ui/Icon";
import css from "./FilesMission.module.css";

// FilesMission is the mission a capsule is for, at the end of its bar: a
// chip that goes there.
export function FilesMission({ to, title }: { to: string; title: string }) {
  return (
    <Link to={to} className={chipClass(undefined, css.mission)} title={`The mission this capsule is for: ${title}`}>
      <Icon name="flag" size={12} />
      <span>{title}</span>
    </Link>
  );
}
