import type { ControlProps } from "./controlProps";
import { shown } from "./sections";
import { SettingsStepper } from "./SettingsStepper";

// Stepper is a setting that's a whole number.
export function Stepper({ s, set }: ControlProps) {
  const n = typeof s.value === "number" ? s.value : Number(s.value) || 0;
  return <SettingsStepper value={n} text={shown(s.value)} label={s.label} onChange={(to) => void set(to)} />;
}
