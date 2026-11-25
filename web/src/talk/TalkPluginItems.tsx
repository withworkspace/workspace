import type { ReactNode } from "react";
import css from "./TalkPluginItems.module.css";

// TalkPluginItems is what a plugin says it'll do, item by item: its
// children are TalkPluginItems.
export function TalkPluginItems({ children }: { children: ReactNode }) {
  return <ol className={css.items}>{children}</ol>;
}
