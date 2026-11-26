import { Link } from "react-router";
import { Summary, SummaryRow } from "../ui/Summary";
import css from "./OrchestratorFacts.module.css";
import { Text } from "../ui/Text";

type OrchestratorFactsProps = {
  // The agent it runs on: its harness and model.
  agent: string;
  // Whether crew it starts start at once, or wait for you.
  auto: boolean;
  // How many crew it may start in a row before it asks you.
  most: number;
};

// OrchestratorFacts is how the orchestrator's set up: its agent, the crew
// it starts, and where it's set.
export function OrchestratorFacts({ agent, auto, most }: OrchestratorFactsProps) {
  return (
    <Summary className={css.facts}>
      <SummaryRow label="Agent">
        <Text mono>{agent}</Text>
      </SummaryRow>
      <SummaryRow label="Starting crew">
        {auto ? "Start at once" : "Wait for you to launch"}; up to {most} in a row, then asks you
      </SummaryRow>
      <SummaryRow label="Settings">
        <Link to="/settings/missions#orchestrator">Yours, in Settings</Link>; team's under{" "}
        <code>[orchestrator]</code> in <code>ws.toml</code>
      </SummaryRow>
    </Summary>
  );
}
