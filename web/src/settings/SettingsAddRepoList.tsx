import type { ReactNode } from "react";
import { ListBox } from "../ui/ListBox";
import { Skeleton } from "../ui/Skeleton";
import css from "./SettingsAddRepoList.module.css";
import { Text } from "../ui/Text";

type SettingsAddRepoListProps = {
  label: string;
  // The one chosen, by its name.
  value?: string;
  onChange: (name: string) => void;
  // With nothing in it: still being asked for, or what's said of there
  // being none.
  loading?: boolean;
  none?: string;
  // A SettingsAddRepoOption each.
  children: ReactNode;
};

// SettingsAddRepoList is an owner's repos on GitHub, to pick one from.
export function SettingsAddRepoList({ label, value, onChange, loading, none, children }: SettingsAddRepoListProps) {
  return (
    <ListBox className={css.list} aria-label={label} value={value} onChange={onChange} empty={loading ? <Skeleton rows={5} /> : none ? <Text as="p" dim className={css.none}>{none}</Text> : null}>
      {children}
    </ListBox>
  );
}
