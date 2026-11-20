import css from "./Chip.module.css";
import { cx } from "./cx";
import { type Tone, toneClass } from "./tone";

type ChipProps = { tone?: Tone } & React.ComponentProps<"span">;

// Chip is a word or two in a pill, beside what it's about: a state, a
// kind, a count of something. Without a tone it's grey.
export function Chip({ tone, className, ...rest }: ChipProps) {
  return <span className={chipClass(tone, className)} {...rest} />;
}

// chipClass is a chip's look for something that isn't a span: a link
// that's a chip.
export function chipClass(tone?: Tone, className?: string): string {
  return cx(css.chip, tone && css.toned, toneClass(tone), className);
}
