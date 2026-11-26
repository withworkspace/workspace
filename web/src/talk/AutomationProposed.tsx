import type { ProposedAutomation } from "../api";
import { AutomationFacts } from "../automations/AutomationFacts";
import { AutomationScript } from "../automations/AutomationScript";
import { DryRunShown } from "../automations/DryRunShown";
import css from "./AutomationProposed.module.css";

// AutomationProposed is a proposed automation, on its card: the user's
// words, when it runs and what it may do, and what it would have done
// over the last 30 days (its dry run). Its script is folded, for whoever
// wants to read how it's written.
export function AutomationProposed({ a, script }: { a: ProposedAutomation; script: string }) {
  return (
    <div className={css.automation}>
      {a.about && <p className={css.about}>{a.about}</p>}
      <AutomationFacts when={a.when} may={a.may}>
        {a.replaces && (
          <>
            <dt>Replaces</dt>
            <dd>the automation {a.name} you have</dd>
          </>
        )}
      </AutomationFacts>
      <DryRunShown dr={a.dryRun} />
      <AutomationScript source={script} />
    </div>
  );
}
