import { Link, type To } from "react-router";
import { Icon } from "../../ui/Icon";
import css from "./MissionWorkStepBundle.module.css";

// MissionWorkStepBundle is a bundle that shows a step, at the end of the
// step's line: its name, going to it in Verify.
export function MissionWorkStepBundle({ id, to, title }: { id: string; to: To; title?: string }) {
  return (
    <Link to={to} className={css.bundle} title={title}>
      <Icon name="list-checks" size={12} />
      {id}
    </Link>
  );
}
