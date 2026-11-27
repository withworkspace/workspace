import css from "./AutomationScript.module.css";

// AutomationScript is an automation's script, as written: folded under
// its summary, or open (open) where it's what the page is for.
export function AutomationScript({ source, open }: { source: string; open?: boolean }) {
  return (
    <details className={css.script} open={open}>
      <summary>The script</summary>
      <pre>{source}</pre>
    </details>
  );
}
