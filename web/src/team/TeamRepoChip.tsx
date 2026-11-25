import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import css from "./TeamRepoChip.module.css";
import { TeamRepoDot } from "./TeamRepoDot";

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
      {pr ? <span className={cx("mono", css.pr)}> #{pr}</span> : null}
    </Chip>
  );
}
