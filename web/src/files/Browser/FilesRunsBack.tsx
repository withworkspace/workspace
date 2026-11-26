import { Link, type To } from "react-router";
import { Icon } from "../../ui/Icon";
import css from "./FilesRunsBack.module.css";

// FilesRunsBack is the way back to the list of runs from the one picked,
// where there's no room for both.
export function FilesRunsBack({ to }: { to: To }) {
  return (
    <Link to={to} replace className={css.back} aria-label="Back to runs" title="Back to runs">
      <Icon name="arrow-left" size={14} />
    </Link>
  );
}
