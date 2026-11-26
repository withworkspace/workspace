import { Link } from "react-router";
import { Callout } from "../ui/Callout";
import { Copyable } from "../ui/Copyable";
import css from "./OrchestratorOff.module.css";

const snippet = "[orchestrator]\nenabled = true";

// OrchestratorOff says the orchestrator's off, and the two ways to turn it
// on: yours in Settings, or everyone's in ws.toml, its lines to copy.
export function OrchestratorOff() {
  return (
    <Callout tone="quiet" icon="pause" title="Off: only you move missions on">
      Turn on for you in <Link to="/settings/missions#orchestrator">Settings</Link>, or for everyone in{" "}
      <code>ws.toml</code>:
      <div className={css.snippet}>
        <code>{snippet}</code>
        <Copyable text={snippet}>Copy</Copyable>
      </div>
    </Callout>
  );
}
