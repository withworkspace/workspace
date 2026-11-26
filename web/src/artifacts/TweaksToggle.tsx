import { Count } from "../ui/Count";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { ToggleButton } from "../ui/ToggleButton";
import { changed, type Told } from "./tweaks";
import { hasTweaks } from "./useTweaks";

// TweaksToggle is the button that shows a frame's tray, or hides it:
// there for a page that declares tweaks. keys is the key that does the
// same, where the page binds one.
export function TweaksToggle({ told, shown, onToggle, keys }: { told: Told | undefined; shown: boolean; onToggle: () => void; keys?: string }) {
  if (!hasTweaks(told)) return null;
  const n = changed(told).length;
  return (
    <ToggleButton variant="ghost" size="small" pressed={shown} onChange={onToggle} title="Show or hide tweaks" keys={keys}>
      <Icon name="sliders" size={13} /> Tweaks
      {n ? <Count>{n}</Count> : null} {keys && <Keys k={keys} />}
    </ToggleButton>
  );
}
