import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./SettingsControl.module.css";

type SettingsControlProps = {
  // Where what's in it sits: at the end (a row's), at the start (a wide
  // row's, under the words), or across (a table's cell).
  align?: "start" | "stretch";
  children: ReactNode;
};

// SettingsControl holds a setting's control and, under it, whose its
// value is.
export function SettingsControl({ align, children }: SettingsControlProps) {
  return <div className={cx(css.control, align === "start" && css.start, align === "stretch" && css.stretch)}>{children}</div>;
}
