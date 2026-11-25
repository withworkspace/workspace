import type { ReactNode } from "react";
import { Empty } from "../ui/Empty";
import type { IconName } from "../ui/Icon";
import css from "./RunPanelIdle.module.css";

// RunPanelIdle is a service that isn't running, where its frame would
// be: what Start does (its children), with the way to its own tab
// (action) where it has one.
export function RunPanelIdle({ icon, title, action, children }: { icon: IconName; title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className={css.idle}>
      <Empty icon={icon} title={title} action={action}>
        {children}
      </Empty>
    </div>
  );
}
