import { Link, type To } from "react-router";
import { Icon } from "../../ui/Icon";
import css from "./MissionVerifyKept.module.css";

// MissionVerifyKept goes to the run ws kept of a check's command: all it
// printed, and what it wrote.
export function MissionVerifyKept({ to }: { to: To }) {
  return (
    <p className={css.kept}>
      <Link to={to}>
        <Icon name="terminal" size={11} /> All it printed, and what it wrote
      </Link>
    </p>
  );
}
