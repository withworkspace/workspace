import { Icon, type IconName } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { setTheme, type ThemeChoice, useTheme } from "../ui/theme";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsRow } from "./SettingsRow";

const themes: [ThemeChoice, IconName, string][] = [
  ["system", "monitor", "System"],
  ["light", "sun", "Light"],
  ["dark", "moon", "Dark"],
];

// ThemeRow is the theme: this is where it's chosen.
export function ThemeRow() {
  const { choice } = useTheme();
  return (
    <SettingsRow label={<Marked text={words.theme.label} />} about={<Marked text={words.theme.about} />} source={<BrowserSource />}>
      <Segmented value={choice} onChange={setTheme} aria-label="Theme">
        {themes.map(([c, icon, label]) => (
          <Segment key={c} value={c}>
            <Icon name={icon} size={13} />
            {label}
          </Segment>
        ))}
      </Segmented>
    </SettingsRow>
  );
}
