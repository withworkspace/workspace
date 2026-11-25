import type { ReactNode } from "react";
import css from "./PluginsNote.module.css";

// PluginsNote is a quiet line under a group of plugins, or at the foot.
export function PluginsNote({ children }: { children: ReactNode }) {
  return <p className={css.note}>{children}</p>;
}
