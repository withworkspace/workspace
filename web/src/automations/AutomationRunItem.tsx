import { Link } from "react-router";
import type { AutomationDid, AutomationRun } from "../api";
import { StatusIcon } from "../ui/StatusIcon";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import { runLook } from "./automationWords";
import css from "./AutomationRunItem.module.css";

// AutomationRunItem is one run of an automation on its page: how it went,
// the event it ran on (its ref, what it says) and when, its per key, what
// it did (each notification, conversation started, wait) and why it
// failed; and the lines it logged.
export function AutomationRunItem({ r }: { r: AutomationRun }) {
  const look = runLook(r.state);
  const did = (r.did ?? []).filter((d) => d.what === "notify" || d.what === "converse" || d.what === "wait");
  return (
    <li className={css.run}>
      <StatusIcon icon={look.icon} tone={look.tone} size={13} title={look.words} className={css.mark} />
      <div className={css.main}>
        <div className={css.top}>
          <span className={css.says}>
            <Text mono dim>
              {r.event}
            </Text>{" "}
            {r.says || <Text dim>an event no longer kept</Text>}
          </span>
          <Time iso={r.at} as="relative" className={css.at} />
        </div>
        <div className={css.state}>
          {look.words}
          {r.key && <Text dim> · for {r.key}</Text>}
        </div>
        {did.length > 0 && (
          <ul className={css.did}>
            {did.map((d, i) => (
              <Did key={i} d={d} />
            ))}
          </ul>
        )}
        {r.error && <div className={css.error}>{r.error}</div>}
        {r.log && r.log.length > 0 && <pre className={css.log}>{r.log.join("\n")}</pre>}
      </div>
    </li>
  );
}

function Did({ d }: { d: AutomationDid }) {
  const half = d.started ? <Text dim> (begun, not known to be done)</Text> : null;
  if (d.what === "notify") {
    return (
      <li>
        Notified you: {d.text || <Text dim>(its words weren't kept)</Text>}
        {half}
      </li>
    );
  }
  if (d.what === "converse") {
    return (
      <li>
        Started{" "}
        {d.conversation ? <Link to={`/conversations/${encodeURIComponent(d.conversation)}`}>conversation {d.conversation}</Link> : "a conversation"}
        {half}
      </li>
    );
  }
  return <li>{d.waiting ? `Waiting ${d.text ?? ""}` : `Waited: ${d.text || "done"}`}</li>;
}
