import { Input } from "../ui/Input";
import css from "./ArtifactTweakText.module.css";

// ArtifactTweakText is a tweak that's a word or a line: a text box the
// width of the tray.
export function ArtifactTweakText({ id, value, onChange }: { id: string; value: string; onChange: (value: string) => void }) {
  return <Input id={id} type="text" className={css.text} value={value} onChange={(e) => onChange(e.target.value)} />;
}
