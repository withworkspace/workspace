import { Button } from "../ui/Button";
import { Keys } from "../ui/Keys";
import css from "./SettingsKeyPick.module.css";

type SettingsKeyPickProps = {
  // The key, or none ("").
  value: string;
  // Waiting for the key to be pressed.
  picking: boolean;
  onPick: () => void;
  onClear: () => void;
  onCancel: () => void;
};

// SettingsKeyPick is one key, chosen by pressing it: the key there is (or
// None), Change to press another, and Clear. While it waits, it says so,
// and esc gives up.
export function SettingsKeyPick({ value, picking, onPick, onClear, onCancel }: SettingsKeyPickProps) {
  if (picking)
    return (
      <span className={css.picking} role="status">
        Press a key
        <Button variant="link" onClick={onCancel}>
          Cancel
        </Button>
      </span>
    );
  return (
    <span className={css.pick}>
      {value ? <Keys k={value} /> : <span className={css.none}>None</span>}
      <Button variant="ghost" size="small" onClick={onPick}>
        {value ? "Change" : "Set"}
      </Button>
      {value && (
        <Button variant="link" onClick={onClear}>
          Clear
        </Button>
      )}
    </span>
  );
}
