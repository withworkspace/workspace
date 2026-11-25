import type { ComponentProps } from "react";
import { cx } from "../ui/cx";
import { Select } from "../ui/Select";
import css from "./SettingsSelect.module.css";

// SettingsSelect is a setting picked from a list: a Select at a row's
// width, or across (stretch) where it's in a table's cell.
export function SettingsSelect({ stretch, className, ...rest }: ComponentProps<typeof Select> & { stretch?: boolean }) {
  return <Select className={cx(css.select, stretch && css.stretch, className)} {...rest} />;
}
