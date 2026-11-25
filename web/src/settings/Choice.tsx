import { Option } from "../ui/Select";
import type { ControlProps } from "./controlProps";
import { shown } from "./sections";
import { SettingsSelect } from "./SettingsSelect";

// Choice is a setting that's one of a list: an agent CLI, or a choice
// with more than can all be on show. What it's at, where the list hasn't
// got it, is offered too.
export function Choice({ s, set, stretch }: ControlProps) {
  return (
    <SettingsSelect stretch={stretch} value={shown(s.value)} onChange={set} aria-label={s.label}>
      {!s.choices?.some((c) => c.value === shown(s.value)) && <Option value={shown(s.value)}>{shown(s.value) || "—"}</Option>}
      {s.choices?.map((c) => (
        <Option key={c.value} value={c.value} title={c.note}>
          {`${c.label ?? c.value}${c.note ? " (not here)" : ""}`}
        </Option>
      ))}
    </SettingsSelect>
  );
}
