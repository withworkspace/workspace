import { Input } from "../ui/Input";
import css from "./ArtifactTweakNumber.module.css";
import { Text } from "../ui/Text";

// ArtifactTweakNumber is a number with no ends: a box to type one in, its
// unit after it. invalid marks what's typed as one it can't be set to.
export function ArtifactTweakNumber({ id, value, unit, invalid, onChange }: { id: string; value: string; unit?: string; invalid?: boolean; onChange: (value: string) => void }) {
  return (
    <div className={css.number}>
      <Input id={id} type="text" inputMode="decimal" value={value} onChange={(e) => onChange(e.target.value)} aria-invalid={invalid} />
      {unit && <Text dim>{unit}</Text>}
    </div>
  );
}
