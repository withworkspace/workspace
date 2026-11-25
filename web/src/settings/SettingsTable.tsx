import type { ReactNode } from "react";
import css from "./SettingsTable.module.css";

// SettingsTable is a list to read on the Settings page, as a table: the
// workspace's repos, its groups. Narrow, its fourth column goes.
export function SettingsTable({ children }: { children: ReactNode }) {
  return <table className={css.table}>{children}</table>;
}
