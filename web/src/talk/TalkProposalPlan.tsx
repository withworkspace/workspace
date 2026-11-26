import { Markdown } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import css from "./TalkProposalPlan.module.css";
import { Prose } from "../ui/Prose";

// TalkProposalPlan is the plan confirmed with a brief, under it: open, to
// be folded away.
export function TalkProposalPlan({ text }: { text: string }) {
  return (
    <details className={cx("fold", css.plan)} open>
      <summary>Plan, confirmed with the brief</summary>
      <Prose className={css.prose}>
        <Markdown text={text} />
      </Prose>
    </details>
  );
}
