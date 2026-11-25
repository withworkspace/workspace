import type { Setting } from "../api";
import { useChromeStore } from "../chrome/store";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Marked } from "./Marked";
import { name, words } from "./sections";
import { SettingsRow } from "./SettingsRow";
import { Source } from "./Source";

// PlacesRow is the sidebar's arrangement: arranged in the sidebar itself,
// which the button turns to. Here, whose it is, and the way back.
export function PlacesRow({ settings }: { settings: Setting[] }) {
  const arrange = useChromeStore((s) => s.arrange);
  const mine = settings.find((s) => s.source === "yours") ?? settings.find((s) => s.source === "team");
  return (
    <SettingsRow label={<Marked text={words.places.label} />} tomlKey={<Marked text={settings.map((s) => (s === settings[0] ? name(s) : s.key)).join(" · ")} />} about={<Marked text={words.places.about} />} source={mine && <Source s={mine} brief />}>
      <Button type="button" onClick={arrange}>
        <Icon name="grip" size={13} />
        Arrange the sidebar…
      </Button>
    </SettingsRow>
  );
}
