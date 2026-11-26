import { Input } from "../../ui/Input";
import css from "./MissionWorkFilter.module.css";

// MissionWorkFilter is the field a capsule's changed files are filtered
// by, at the end of their heading.
export function MissionWorkFilter({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return <Input type="text" className={css.filter} placeholder="Filter" aria-label="Filter changed files" value={value} onChange={(e) => onChange(e.target.value)} />;
}
