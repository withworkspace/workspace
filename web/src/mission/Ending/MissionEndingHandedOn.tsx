import { Link } from "react-router";
import css from "./MissionEndingHandedOn.module.css";
import { Text, textClass } from "../../ui/Text";

// MissionEndingHandedOn names the missions a mission was handed on to,
// each a link; with none, nothing.
export function MissionEndingHandedOn({ to }: { to?: string[] }) {
  if (!to?.length) return null;
  return (
    <Text as="div" dim className={css.handedOn}>
      Handed on to{" "}
      {to.map((s, i) => (
        <span key={s}>
          {i > 0 && ", "}
          <Link to={`/missions/${s}`} className={textClass("link", "mono")}>
            {s}
          </Link>
        </span>
      ))}
    </Text>
  );
}
