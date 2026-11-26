import { Callout } from "../ui/Callout";
import css from "./SettingsUnknown.module.css";

// SettingsUnknown is the keys either file has that ws doesn't read, each
// said with its file.
export function SettingsUnknown({ keys }: { keys: string[] }) {
  return (
    <Callout tone="attention" title="Unknown keys">
      Misspelt or in the wrong file, so ignored:
      <ul className={css.list}>
        {keys.map((u) => (
          <li key={u}>
            <code>{u}</code>
          </li>
        ))}
      </ul>
    </Callout>
  );
}
