import { Callout, type CalloutTone } from "../ui/Callout";
import { Clamp } from "../ui/Clamp";
import type { IconName } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import css from "./MissionPeekStand.module.css";

export type MissionPeekStandInfo = {
  // Whose it is, as a tone: yours, an agent's, outside's, done.
  tone: CalloutTone;
  icon: IconName;
  // What's happening, or what it waits on.
  title: string;
  // The sentence under it, as crew or ws wrote it.
  detail?: string;
  // It asks nothing of you.
  nothing?: boolean;
};

// MissionPeekStand is one thing a mission stands on, read and not acted
// on: what it waits on and whose that is, as its own page's cards say it,
// without what they offer. What's done about it is done on its page.
export function MissionPeekStand({ tone, icon, title, detail, nothing, lead }: MissionPeekStandInfo & { lead?: boolean }) {
  return (
    <Callout tone={lead ? tone : "quiet"} icon={icon} title={title} className={css.stand}>
      {detail && (
        <Clamp lines={4} more="More" less="Less" of={detail}>
          <Inline text={detail} />
        </Clamp>
      )}
      {nothing && <span className={css.nothing}>Nothing for you to do</span>}
    </Callout>
  );
}
