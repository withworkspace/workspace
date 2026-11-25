import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SettingsRepoGroup.module.css";

type SettingsRepoGroupProps = {
  // The pick: a SettingsGroupPick.
  children: ReactNode;
  // Where yours is over the team's: what the team has it in, said on
  // hover.
  over?: string;
};

// SettingsRepoGroup is a repo's group in the repos table: the pick, and
// "yours" where it's yours over the team's.
export function SettingsRepoGroup({ children, over }: SettingsRepoGroupProps) {
  return (
    <span className={css.cell}>
      {children}
      {over && (
        <span className={cx("dim", css.over)} title={over}>
          yours
        </span>
      )}
    </span>
  );
}
