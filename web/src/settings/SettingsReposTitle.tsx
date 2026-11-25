import { Button } from "../ui/Button";
import { Count } from "../ui/Count";
import css from "./SettingsReposTitle.module.css";

// SettingsReposTitle is the repos block's heading: how many there are,
// and the way to add one.
export function SettingsReposTitle({ count, onAdd }: { count?: number; onAdd: () => void }) {
  return (
    <>
      Repos
      {count !== undefined && <Count>{count}</Count>}
      <Button size="small" className={css.add} onClick={onAdd}>
        Add repo…
      </Button>
    </>
  );
}
