import { cx } from "../ui/cx";
import css from "./SettingsProcessNone.module.css";

// SettingsProcessNone is said on the Settings page where the workspace
// has no process yet: where to write one.
export function SettingsProcessNone({ name }: { name: string }) {
  return (
    <div className={cx(css.none, "dim")}>
      There's no process yet: write how work goes here in <code>{name}</code>, beside <code>ws.toml</code> (or name another file with <code>[agents] process</code>).
    </div>
  );
}
