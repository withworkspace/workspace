import { Option, Select } from "../ui/Select";
import type { Choice } from "./tweaks";

// ArtifactTweakSelect is a tweak that's one of many, as a dropdown the
// width of the tray: a Select, with no look of its own.
export function ArtifactTweakSelect({ id, label, value, options, onChange }: { id: string; label: string; value: string; options: Choice[]; onChange: (value: string) => void }) {
  return (
    <div id={id}>
      <Select fill size="small" value={value} onChange={onChange} aria-label={label}>
        {options.map((o) => (
          <Option key={o.value} value={o.value} title={o.description || undefined}>
            {o.label}
          </Option>
        ))}
      </Select>
    </div>
  );
}
