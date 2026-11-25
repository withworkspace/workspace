import { Markdown } from "../../markdown/LazyMarkdown";
import { Clamp } from "../../ui/Clamp";
import css from "./MissionSummarySaid.module.css";

// MissionSummarySaid is what crew last said, drawn as it was written, a
// few lines of it until you ask for the rest.
export function MissionSummarySaid({ text }: { text: string }) {
  return (
    <Clamp className={css.said} of={text}>
      <Markdown text={text} />
    </Clamp>
  );
}
