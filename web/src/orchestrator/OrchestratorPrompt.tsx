import { Markdown } from "../markdown/LazyMarkdown";
import css from "./OrchestratorPrompt.module.css";
import { Prose } from "../ui/Prose";

// OrchestratorPrompt is the prompt a run was given, folded: it's long, and
// the same each time but for the mission.
export function OrchestratorPrompt({ text }: { text: string }) {
  return (
    <details className={css.fold}>
      <summary>Prompt</summary>
      <Prose className={css.prompt}>
        <Markdown text={text} />
      </Prose>
    </details>
  );
}
