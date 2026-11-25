import { Markdown } from "../markdown/LazyMarkdown";
import css from "./TalkSideSummary.module.css";

// TalkSideSummary is a conversation's summary so far, beside it.
export function TalkSideSummary({ text }: { text: string }) {
  return (
    <div className={css.summary}>
      <Markdown text={text} />
    </div>
  );
}
