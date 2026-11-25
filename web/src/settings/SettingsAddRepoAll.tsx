import { Checkbox } from "../ui/Checkbox";
import { cx } from "../ui/cx";
import css from "./SettingsAddRepoAll.module.css";

// SettingsAddRepoAll shows the forks and archived repos too, which the
// list leaves out: how many there are, and the box that brings them in.
export function SettingsAddRepoAll({ hidden, checked, onChange }: { hidden: number; checked: boolean; onChange: (on: boolean) => void }) {
  return (
    <Checkbox className={cx("dim", css.all)} checked={checked} onChange={onChange}>
      Show forks and archived repos ({hidden})
    </Checkbox>
  );
}
