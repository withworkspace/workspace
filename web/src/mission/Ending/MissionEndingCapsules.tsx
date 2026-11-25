import { Link } from "react-router";
import { Icon } from "../../ui/Icon";
import css from "./MissionEndingCapsules.module.css";

// A capsule that ended alone: dropped (and why, if it was said), or handed
// on to a mission of its own.
export type EndingCapsule = { repo: string; name: string; why?: string; to?: string; dropped: boolean };

// MissionEndingCapsules are the capsules that ended alone: no longer the
// mission's, and not holding it open. Each says why on hover.
export function MissionEndingCapsules({ capsules }: { capsules: EndingCapsule[] }) {
  return (
    <ul className={css.list}>
      {capsules.map((c) => (
        <li key={`${c.repo}/${c.name}`} title={c.why}>
          <Icon name={c.dropped ? "x" : "arrow-right"} size={12} className="dim" />
          <span className="mono">
            {c.repo}/{c.name}
          </span>{" "}
          {c.dropped ? (
            <span className="dim">dropped{c.why ? `: ${c.why}` : ""}</span>
          ) : (
            <span className="dim">
              handed on to{" "}
              <Link to={`/missions/${c.to}`} className="link mono">
                {c.to}
              </Link>
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
