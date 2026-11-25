import type { Setting } from "../api";
import { Control } from "./Control";
import { Marked } from "./Marked";
import { name } from "./sections";
import { SettingsRow } from "./SettingsRow";
import { Source } from "./Source";
import { useSetting } from "./useSetting";

// KeyRow is the row of a setting a file has a key for: what it is, its
// control by its kind, and whose its value is. crumb is the section it's
// from, said where the row is out of its block (Find's results).
export function KeyRow({ s, crumb }: { s: Setting; crumb?: string }) {
  const { set, reset } = useSetting(s);
  return (
    <SettingsRow label={<Marked text={s.label} />} tomlKey={<Marked text={name(s)} />} crumb={crumb} about={s.about && <Marked text={s.about} />} wide={s.kind === "picks"} source={<Source s={s} />}>
      <Control s={s} set={set} reset={reset} />
    </SettingsRow>
  );
}
