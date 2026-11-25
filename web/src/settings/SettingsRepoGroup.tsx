import { cx } from "../ui/cx";
import type { ReactNode } from "react";
import css from "./SettingsRepoGroup.module.css";
import { Text } from "../ui/Text";
import { settingsTableQuietClass } from "./SettingsTable";

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
        <Text dim className={cx(css.over, settingsTableQuietClass)} title={over}>
          yours
        </Text>
      )}
    </span>
  );
}
