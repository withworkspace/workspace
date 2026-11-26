import { Link } from "react-router";
import { Status, type StatusInfo } from "../ui/Status";
import css from "./TeamPulls.module.css";
import { TeamRepoDot } from "./TeamRepoDot";
import { TeamState } from "./TeamState";
import { Text } from "../ui/Text";

export type TeamPull = {
  key: string;
  status: StatusInfo;
  repo: string;
  color?: string;
  number: number;
  // Where it goes: reviewed here (to), or on GitHub (href), or nowhere.
  to?: string;
  href?: string;
};

// TeamPulls is a change's pull requests, where it was made in several
// repos: each where it stands, its repo and number, and where it goes.
export function TeamPulls({ pulls }: { pulls: TeamPull[] }) {
  return (
    <ul className={css.pulls}>
      {pulls.map((p) => {
        const inner = (
          <>
            <Status s={p.status} iconOnly size={13} />
            <TeamRepoDot color={p.color} />
            <span>{p.repo}</span>
            <Text mono dim>#{p.number}</Text>
            <TeamState tone={p.status.tone}>{p.status.words}</TeamState>
          </>
        );
        return (
          <li key={p.key}>
            {p.to ? (
              <Link to={p.to} title="Review it here">
                {inner}
              </Link>
            ) : p.href ? (
              <a href={p.href} target="_blank" rel="noreferrer" title="Open on GitHub">
                {inner}
              </a>
            ) : (
              <span>{inner}</span>
            )}
          </li>
        );
      })}
    </ul>
  );
}
