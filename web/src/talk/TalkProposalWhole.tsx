import { Markdown } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import css from "./TalkProposalWhole.module.css";

// TalkProposalWhole is what confirming a card commits you to (a mission's
// brief, a document), shown whole: it isn't folded away.
export function TalkProposalWhole({ text }: { text: string }) {
  return (
    <div className={cx("prose", css.whole)}>
      <Markdown text={text} />
    </div>
  );
}
