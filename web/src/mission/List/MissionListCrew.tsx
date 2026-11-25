import { Chip } from "../../ui/Chip";
import { Icon, type IconName } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import type { Tone } from "../../ui/tone";
import css from "./MissionListCrew.module.css";

// MissionListCrew is one of a mission's crew at work, in its row of the
// missions list: its id in a chip in its tone, with its status's icon
// (pulsing, or turning, while it's at it). Hidden on a narrow window.
export function MissionListCrew({ id, tone, icon, live, says }: { id: string; tone?: Tone; icon: IconName; live?: boolean; says: string }) {
  return (
    <Chip tone={tone} className={css.chip} title={`${id}: ${says}`}>
      {live && icon === "spinner" ? <Spinner size={12} /> : <Icon name={icon} size={12} className={live ? css.pulse : undefined} />}
      {id}
    </Chip>
  );
}
