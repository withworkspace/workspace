import type { AutomationDryLine, AutomationDryRun } from "../api";
import { Text } from "../ui/Text";
import css from "./DryRunShown.module.css";

// DryRunShown is what an automation would have done over the last 30
// days (its dry run): how many events were for it and how many runs it'd
// have had, then each run with what it'd have done, what its waits came
// to, or why it'd have been held back or failed. On a proposed one's card,
// and on its page when it's dry-run again.
export function DryRunShown({ dr }: { dr: AutomationDryRun }) {
  const lines = dr.lines ?? [];
  const counted = [
    `${dr.matched} of ${dr.events} events ${dr.matched === 1 ? "is" : "are"} for it`,
    `it would have run ${dr.ran} ${dr.ran === 1 ? "time" : "times"}`,
    dr.held ? `${dr.held} held back` : "",
    dr.waiting ? `${dr.waiting} waiting still` : "",
    dr.settling ? `${dr.settling} settling still` : "",
    dr.failed ? `${dr.failed} failing` : "",
  ].filter(Boolean);
  return (
    <div className={css.dry}>
      <div className={css.head}>
        Over the last 30 days: <Text dim>{counted.join(" · ")}</Text>
      </div>
      {lines.length > 0 ? (
        <ol className={css.lines}>
          {lines.map((l) => (
            <DryLine key={l.event + l.at} l={l} />
          ))}
        </ol>
      ) : (
        <Text as="div" dim>
          Nothing it's for happened: it'd have done nothing yet.
        </Text>
      )}
    </div>
  );
}

function DryLine({ l }: { l: AutomationDryLine }) {
  const did = l.held ? [`held back: ${l.held}`] : [...(l.waited ?? []), ...(l.did ?? []), ...(l.error ? [`failed: ${l.error}`] : [])];
  return (
    <li className={l.error ? css.failed : l.held ? css.held : undefined}>
      <span className={css.event}>
        <Text mono dim>
          {l.event}
        </Text>{" "}
        {l.says}
        {l.batch ? <Text dim> ({l.batch} together)</Text> : null}
      </span>
      {did.length > 0 ? (
        <ul className={css.did}>
          {did.map((d, i) => (
            <li key={i}>{d}</li>
          ))}
        </ul>
      ) : (
        <Text as="div" dim className={css.did}>
          nothing
        </Text>
      )}
    </li>
  );
}
