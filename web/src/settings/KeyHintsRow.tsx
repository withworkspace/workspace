import { usePref } from "../prefs";
import { Switch } from "../ui/Switch";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsRow } from "./SettingsRow";

// KeyHintsRow is key hints, on or off (ui/keyHint.ts): being told that
// what you pressed with the pointer has a key. Off unless it's turned on
// here.
export function KeyHintsRow() {
  const [on, setOn] = usePref("keyHints");
  return (
    <SettingsRow label={<Marked text={words.keyHints.label} />} about={<Marked text={words.keyHints.about} />} source={<BrowserSource />}>
      <Switch on={on} onChange={setOn} aria-label={words.keyHints.label} />
    </SettingsRow>
  );
}
