import type { Setting } from "../api";
import { Control } from "./Control";
import { SettingsControl } from "./SettingsControl";
import { Source } from "./Source";
import { useSetting } from "./useSetting";

// KeyCell is a setting as a table's cell: its control, across the cell,
// and whose it is.
export function KeyCell({ s }: { s: Setting }) {
  const { set, reset } = useSetting(s);
  return (
    <SettingsControl align="stretch">
      <Control s={s} set={set} reset={reset} stretch />
      <Source s={s} brief />
    </SettingsControl>
  );
}
