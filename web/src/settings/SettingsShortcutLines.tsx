import { Icon } from "../ui/Icon";
import css from "./SettingsShortcutLines.module.css";

// SettingsShortcutLines is what's said under a shortcut of its keys: what
// they clash with (each a line, marked), then what's so but no trouble
// (notes, quieter).
export function SettingsShortcutLines({ clashes, notes }: { clashes: readonly string[]; notes: readonly string[] }) {
  if (clashes.length === 0 && notes.length === 0) return null;
  return (
    <ul className={css.lines}>
      {clashes.map((c) => (
        <li key={c} className={css.clash}>
          <Icon name="alert" /> {c}
        </li>
      ))}
      {notes.map((n) => (
        <li key={n} className={css.note}>
          {n}
        </li>
      ))}
    </ul>
  );
}
