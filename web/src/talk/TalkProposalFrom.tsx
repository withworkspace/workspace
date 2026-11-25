import css from "./TalkProposalFrom.module.css";
import { Text } from "../ui/Text";

// TalkProposalFrom says which plugin a proposal came through, and what it
// proposes as (plugin:proposes), which does what the card says once it's
// accepted.
export function TalkProposalFrom({ plugin, proposes }: { plugin: string; proposes?: string }) {
  return (
    <Text as="div" dim className={css.from}>
      Proposed through the {plugin} plugin ({plugin}:{proposes}),
      which does what it says here once you accept it.
    </Text>
  );
}
