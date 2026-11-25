import { Markdown } from "../markdown/LazyMarkdown";
import { Icon } from "../ui/Icon";
import css from "./TalkSummary.module.css";

// TalkSummary is a conversation's summary so far, folded, over its turns.
export function TalkSummary({ text }: { text: string }) {
  return (
    <details className={css.summary}>
      <summary>
        <Icon name="chevron-right" size={12} className={css.chevron} /> Summary so far
      </summary>
      <Markdown text={text} />
    </details>
  );
}
