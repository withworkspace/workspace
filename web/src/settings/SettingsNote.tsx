import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SettingsNote.module.css";

// SettingsNote is more about what a control's at, under it: why a
// harness's models couldn't be listed, how the theme shows the accent.
// left is for one in a table's cell, which reads from the start.
export function SettingsNote({ left, children }: { left?: boolean; children: ReactNode }) {
  return <span className={cx(css.note, left && css.left)}>{children}</span>;
}
