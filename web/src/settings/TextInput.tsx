import type { ControlProps } from "./controlProps";
import { shown } from "./sections";
import { SettingsField } from "./SettingsField";

// TextInput is a setting that's typed. blank starts it empty, with the
// keys (another model, by name). done is told once it's left, whatever
// came of it.
export function TextInput({ s, set, stretch, short, placeholder, blank, done }: ControlProps & { short?: boolean; placeholder?: string; blank?: boolean; done?: () => void }) {
  // A model that's the default is said by the placeholder, the field empty.
  const was = blank || (s.source === "default" && (s.kind === "model" || s.kind === "level")) ? "" : shown(s.value);
  return <SettingsField value={was} label={s.label} placeholder={placeholder} short={short} mono={s.kind === "model" || s.kind === "level"} stretch={stretch} focused={blank} onCommit={(t) => void set(t)} onDone={done} />;
}
