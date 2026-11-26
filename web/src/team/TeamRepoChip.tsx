import { Chip } from "../ui/Chip";
import css from "./TeamRepoChip.module.css";
import { TeamRepoDot } from "./TeamRepoDot";
import { Text } from "../ui/Text";

type TeamRepoChipProps = {
  // Its repos' colours: one, or a few of a change made in several.
  colors: (string | undefined)[];
  label: string;
  // The number of its pull request, after the label.
  pr?: number;
  title?: string;
};

// TeamRepoChip is where a change on the Team page was made: its repo's
// colour and name (and its pull request's number), or, for one made in
// several, their colours over each other and how many.
export function TeamRepoChip({ colors, label, pr, title }: TeamRepoChipProps) {
  return (
    <Chip tone="quiet" className={css.chip} title={title}>
      {colors.map((c, i) => (
        <TeamRepoDot key={i} color={c} className={css.dot} />
      ))}
      {label}
      {pr ? <Text mono className={css.pr}> #{pr}</Text> : null}
    </Chip>
  );
}
