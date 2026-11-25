import { cx } from "../../ui/cx";
import { Icon, type IconName } from "../../ui/Icon";
import type { ActorKind } from "../eventView";
import css from "./MissionLogActor.module.css";

const icons: Record<ActorKind, IconName> = { you: "user", crew: "terminal", orchestrator: "sparkle", agent: "message", ws: "circle" };
const looks: Partial<Record<ActorKind, string>> = { you: css.you, orchestrator: css.orchestrator, agent: css.agent, ws: css.ws };

// MissionLogActor is who did something in the log, when it isn't crew
// (crew are their badges): you, the orchestrator, an agent, or ws, each
// a mark in its own colour, named on hover.
export function MissionLogActor({ actor, who }: { actor: ActorKind; who: string }) {
  return (
    <span className={cx(css.actor, looks[actor])} title={who}>
      <Icon name={icons[actor]} size={12} />
    </span>
  );
}
