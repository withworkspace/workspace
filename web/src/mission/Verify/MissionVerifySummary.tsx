import { Clamp } from "../../ui/Clamp";
import { Inline } from "../../ui/Inline";
import css from "./MissionVerifySummary.module.css";

// MissionVerifySummary is what a bundle's crew say of it, a few lines of
// it until you ask for the rest: its checks are what you came for.
export function MissionVerifySummary({ text }: { text: string }) {
  return (
    <Clamp className={css.summary} tall={7.6} more="More" less="Less" of={text}>
      {text.split(/\n\s*\n/).map((para, i) => (
        <p key={i}>
          <Inline text={para} />
        </p>
      ))}
    </Clamp>
  );
}
