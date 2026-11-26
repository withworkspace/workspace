import type { ProposedAutomation } from "../api";
import { AutomationFacts } from "../automations/AutomationFacts";
import { AutomationFlow } from "../automations/AutomationFlow";
import { AutomationScript } from "../automations/AutomationScript";
import { DryRunShown } from "../automations/DryRunShown";
import css from "./AutomationProposed.module.css";

// AutomationProposed is a proposed automation, on its card: the user's
// words, what it may do, its script drawn as what it does (when it runs,
// then its steps, each with how often it would have over the last 30
// days), and its dry run's runs. Its script is folded, for whoever
// wants to read how it's written.
export function AutomationProposed({ a, script }: { a: ProposedAutomation; script: string }) {
  return (
    <div className={css.automation}>
      {a.about && <p className={css.about}>{a.about}</p>}
      <AutomationFacts when={a.outline ? undefined : a.when} may={a.may}>
        {a.replaces && (
          <>
            <dt>Replaces</dt>
            <dd>the automation {a.name} you have</dd>
          </>
        )}
      </AutomationFacts>
      {a.outline && <AutomationFlow outline={a.outline} steps={a.dryRun.steps} failures={a.dryRun.failures} dry />}
      <DryRunShown dr={a.dryRun} />
      <AutomationScript source={script} />
    </div>
  );
}
