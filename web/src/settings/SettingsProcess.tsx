import type { ReactNode } from "react";
import css from "./SettingsProcess.module.css";

type SettingsProcessProps = {
  // The file's name, and the commit it's as (or changed since), if any.
  name: string;
  commit?: string;
  uncommitted?: boolean;
  // The file, drawn as it's written.
  children: ReactNode;
};

// SettingsProcess is the workspace's process on the Settings page: which
// file it is and whether it's as committed, over the file as it's written.
export function SettingsProcess({ name, commit, uncommitted, children }: SettingsProcessProps) {
  return (
    <>
      <div className={css.bar}>
        <span className={css.where}>
          <code>{name}</code>
          <span className="dim">
            {commit && !uncommitted && <> · as committed in {commit}</>}
            {commit && uncommitted && <> · changed since {commit}, not committed</>}
            {!commit && <> · not committed</>}
          </span>
        </span>
      </div>
      <div className={css.file}>
        <div className="prose">{children}</div>
      </div>
    </>
  );
}
