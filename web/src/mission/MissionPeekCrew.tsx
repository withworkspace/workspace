import { Link } from "react-router";
import { Status, type StatusInfo } from "../ui/Status";
import { MissionCrewBadge } from "./MissionCrewBadge";
import css from "./MissionPeekCrew.module.css";

export type MissionPeekCrewInfo = {
  id: string;
  // How it stands: working, waiting for you, asking.
  status: StatusInfo;
  // The agent it runs on: "claude on opus".
  what: string;
  // The capsule it's in, repo/name.
  capsule?: string;
  // Its own page: its transcript, or its terminal.
  to: string;
};

// MissionPeekCrew is one of a mission's crew that hasn't ended: its
// badge, how it stands, and what it is, as a way in to watch it.
export function MissionPeekCrew({ id, status, what, capsule, to }: MissionPeekCrewInfo) {
  return (
    <li className={css.crew}>
      <Link to={to} className={css.link}>
        <MissionCrewBadge id={id} live={status.live} />
        <Status s={status} className={css.status} />
        <span className={css.what}>
          {what}
          {capsule && <span className={css.capsule}> in {capsule}</span>}
        </span>
      </Link>
    </li>
  );
}
