import { Time } from "../ui/Time";
import css from "./OrchestratorRunMeta.module.css";

type OrchestratorRunMetaProps = {
  // What it looked at ("c1 (done)").
  on: string;
  started: string;
  // How long it took, in words.
  took: string;
  agent: string;
};

// OrchestratorRunMeta is a run's facts under its page's head: what it
// looked at, when and for how long, and on which agent.
export function OrchestratorRunMeta({ on, started, took, agent }: OrchestratorRunMetaProps) {
  return (
    <div className={css.meta}>
      <span>
        Looked at <strong>{on}</strong>
      </span>
      <span>
        <Time iso={started} /> · took {took}
      </span>
      <span className="mono">{agent}</span>
    </div>
  );
}
