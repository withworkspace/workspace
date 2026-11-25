import { Switch } from "../ui/Switch";
import css from "./ArtifactTweakSwitch.module.css";

// ArtifactTweakSwitch is a tweak that's on or off: a Switch, at the end of
// its label's line.
export function ArtifactTweakSwitch({ id, label, on, onChange }: { id: string; label: string; on: boolean; onChange: (on: boolean) => void }) {
  return (
    <span id={id}>
      <Switch className={css.toggle} on={on} aria-label={label} onChange={onChange} />
    </span>
  );
}
