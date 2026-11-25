import { Button } from "../../ui/Button";
import { Callout } from "../../ui/Callout";
import css from "./PluginsFailed.module.css";

// PluginsFailed is what didn't work, said above the list: a sentence, and
// what a build printed on the way, where it got that far.
export function PluginsFailed({ what, error, output, onDismiss }: { what: string; error: string; output?: string; onDismiss: () => void }) {
  return (
    <Callout tone="danger" title={what} actions={<Button onClick={onDismiss}>Dismiss</Button>}>
      {error}
      {output && <pre className={css.output}>{output}</pre>}
    </Callout>
  );
}
