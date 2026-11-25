import type { ReactNode } from "react";
import { Rows } from "../../ui/Rows";
import css from "./PluginsList.module.css";

// PluginsList is a list of plugins, each a PluginsRow.
export function PluginsList({ children }: { children: ReactNode }) {
  return <Rows className={css.list}>{children}</Rows>;
}
