import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import { Status, type StatusInfo } from "../ui/Status";
import css from "./MissionPeekCapsule.module.css";

export type MissionPeekCapsuleInfo = {
  // repo/name.
  name: string;
  // It's been burned, or was never lifted here.
  gone?: boolean;
  // Its pull request: its number as it's written (api#46), where its
  // review is here, and how it stands.
  pr?: { name: string; to: string; status: StatusInfo };
};

// MissionPeekCapsule is one of the capsules a mission's work is in, and
// the pull request it's become, which goes to its review here.
export function MissionPeekCapsule({ name, gone, pr }: MissionPeekCapsuleInfo) {
  return (
    <li className={css.capsule}>
      <Icon name="branch" size={13} className={css.icon} />
      <span className={gone ? css.gone : css.name} title={gone ? `${name}: no longer exists` : name}>
        {name}
      </span>
      {pr && (
        <Link to={pr.to} className={css.pr}>
          <Status s={pr.status} iconOnly size={13} />
          {pr.name}
        </Link>
      )}
    </li>
  );
}
