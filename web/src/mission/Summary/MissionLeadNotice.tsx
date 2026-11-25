import type { ReactNode } from "react";
import { Callout } from "../../ui/Callout";
import type { IconName } from "../../ui/Icon";
import css from "./MissionLeadNotice.module.css";

// MissionLeadNotice is a mission's card after its first: a notice under
// the lead, quiet, with what it offers.
export function MissionLeadNotice({ icon, title, actions, children }: { icon: IconName; title: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return (
    <Callout tone="quiet" icon={icon} title={title} className={css.notice} big actions={actions}>
      {children}
    </Callout>
  );
}
