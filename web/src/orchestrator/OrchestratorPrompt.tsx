import { Markdown } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import css from "./OrchestratorPrompt.module.css";

// OrchestratorPrompt is the prompt a run was given, folded: it's long, and
// the same each time but for the mission.
export function OrchestratorPrompt({ text }: { text: string }) {
  return (
    <details className={css.fold}>
      <summary>The prompt it was given</summary>
      <div className={cx("prose", css.prompt)}>
        <Markdown text={text} />
      </div>
    </details>
  );
}
