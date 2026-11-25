import { Count } from "../ui/Count";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { ToggleButton } from "../ui/ToggleButton";
import { changed, type Told } from "./tweaks";
import { hasTweaks } from "./useTweaks";

// TweaksToggle is the button that shows a frame's tray, or hides it:
// there for a page that declares tweaks.
export function TweaksToggle({ told, shown, onToggle }: { told: Told | undefined; shown: boolean; onToggle: () => void }) {
  if (!hasTweaks(told)) return null;
  const n = changed(told).length;
  return (
    <ToggleButton variant="ghost" size="small" pressed={shown} onChange={onToggle} title="What its page says can be changed about it">
      <Icon name="sliders" size={13} /> Tweaks
      {n ? <Count>{n}</Count> : null} <Keys k="t" />
    </ToggleButton>
  );
}
