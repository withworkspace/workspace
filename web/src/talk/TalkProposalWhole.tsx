import { Markdown } from "../markdown/LazyMarkdown";
import css from "./TalkProposalWhole.module.css";
import { Prose } from "../ui/Prose";

// TalkProposalWhole is what confirming a card commits you to (a mission's
// brief, a document), shown whole: it isn't folded away.
export function TalkProposalWhole({ text }: { text: string }) {
  return (
    <Prose className={css.whole}>
      <Markdown text={text} />
    </Prose>
  );
}
