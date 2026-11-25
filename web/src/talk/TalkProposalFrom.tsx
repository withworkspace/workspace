import { cx } from "../ui/cx";
import css from "./TalkProposalFrom.module.css";

// TalkProposalFrom says which plugin a proposal came through, and what it
// proposes as (plugin:proposes), which does what the card says once it's
// accepted.
export function TalkProposalFrom({ plugin, proposes }: { plugin: string; proposes?: string }) {
  return (
    <div className={cx("dim", css.from)}>
      Proposed through the {plugin} plugin ({plugin}:{proposes}),
      which does what it says here once you accept it.
    </div>
  );
}
