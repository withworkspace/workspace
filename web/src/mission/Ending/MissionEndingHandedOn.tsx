import { Link } from "react-router";
import { cx } from "../../ui/cx";
import css from "./MissionEndingHandedOn.module.css";

// MissionEndingHandedOn names the missions a mission was handed on to,
// each a link; with none, nothing.
export function MissionEndingHandedOn({ to }: { to?: string[] }) {
  if (!to?.length) return null;
  return (
    <div className={cx(css.handedOn, "dim")}>
      Handed on to{" "}
      {to.map((s, i) => (
        <span key={s}>
          {i > 0 && ", "}
          <Link to={`/missions/${s}`} className="link mono">
            {s}
          </Link>
        </span>
      ))}
    </div>
  );
}
