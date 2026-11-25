import type { ControlProps } from "./controlProps";
import { SettingsPicks } from "./SettingsPicks";

// Picks is a setting that's a list (of harness:model). Yours can't be an
// empty list (that is no list: the team's, or none), so taking your last
// one off resets it.
export function Picks({ s, set, reset }: ControlProps) {
  const picks = Array.isArray(s.value) ? (s.value as string[]) : [];
  const drop = (i: number) => {
    const left = picks.filter((_, j) => j !== i);
    if (left.length === 0 && s.source === "yours") reset();
    else void set(left);
  };
  return <SettingsPicks picks={picks} label={s.label} example="claude:sonnet" onAdd={(more) => void set([...picks, ...more])} onDrop={drop} />;
}
