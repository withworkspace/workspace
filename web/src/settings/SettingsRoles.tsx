import type { ReactNode } from "react";
import css from "./SettingsRoles.module.css";

// SettingsRoles is every role's agent in one table: the agent CLI it runs
// in, and the model it's given, a SettingsRole each.
export function SettingsRoles({ children }: { children: ReactNode }) {
  return (
    <div>
      <div className={css.head}>
        <span>Role</span>
        <span>Agent</span>
        <span>Model</span>
      </div>
      {children}
    </div>
  );
}
