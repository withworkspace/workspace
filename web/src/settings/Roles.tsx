import type { SettingsGroup } from "../api";
import { KeyCell } from "./KeyCell";
import { SettingsRole } from "./SettingsRole";
import { SettingsRoles } from "./SettingsRoles";

// Roles is every role's agent in one table: the agent CLI it runs in, and
// the model it's given, with why it can't run if it can't, and what it
// follows where it isn't set itself.
export function Roles({ roles }: { roles: SettingsGroup[] }) {
  return (
    <SettingsRoles>
      {roles.map((g) => {
        const harness = g.settings.find((s) => s.kind === "harness");
        const model = g.settings.find((s) => s.kind === "model");
        return <SettingsRole key={g.id} title={g.title} id={g.id} about={g.about} problem={g.runs?.problem} note={g.runs?.note} agent={harness && <KeyCell s={harness} />} model={model && <KeyCell s={model} />} />;
      })}
    </SettingsRoles>
  );
}
