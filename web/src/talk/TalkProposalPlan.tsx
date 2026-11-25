import { Markdown } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import css from "./TalkProposalPlan.module.css";

// TalkProposalPlan is the plan confirmed with a brief, under it: open, to
// be folded away.
export function TalkProposalPlan({ text }: { text: string }) {
  return (
    <details className={cx("fold", css.plan)} open>
      <summary>The plan, confirmed with it</summary>
      <div className="prose">
        <Markdown text={text} />
      </div>
    </details>
  );
}
