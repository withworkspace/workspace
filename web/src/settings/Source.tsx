import type { Setting } from "../api";
import { valueSaid } from "./sections";
import { SettingsSource } from "./SettingsSource";
import { has, useSetting, without } from "./useSetting";

// Source says where a setting's value comes from, when it isn't ws's
// default: the team's file, or yours, with what it's over and the way
// back. brief says only whose (a table's cell: the rest is its title).
export function Source({ s, brief }: { s: Setting; brief?: boolean }) {
  const { reset } = useSetting(s);
  if (s.source === "default") return null;
  if (s.source === "team") return <SettingsSource title="Set in the team's ws.toml">the team's</SettingsSource>;
  const over = has(s.team) ? `the team's is ${valueSaid(s, s.team)}` : has(s.default) ? `default ${valueSaid(s, s.default)}` : "";
  return (
    <SettingsSource yours title={`From your ws.local.toml${brief && over ? `: ${over}` : ""}`} onReset={reset} resetTitle={`Reset to ${without(s)}`}>
      yours{!brief && over ? ` · ${over}` : ""}
    </SettingsSource>
  );
}
