import css from "./SettingsProcessNone.module.css";
import { Text } from "../ui/Text";

// SettingsProcessNone is said on the Settings page where the workspace
// has no process yet: where to write one.
export function SettingsProcessNone({ name }: { name: string }) {
  return (
    <Text as="div" dim className={css.none}>
      No process yet: describe how work goes in <code>{name}</code>, beside <code>ws.toml</code> (or set
      another file with <code>[agents] process</code>).
    </Text>
  );
}
