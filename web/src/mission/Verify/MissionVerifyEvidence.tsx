import type { ReactNode } from "react";
import css from "./MissionVerifyEvidence.module.css";
import { MissionVerifyCommand } from "./MissionVerifyCommand";

type MissionVerifyEvidenceProps = {
  // The file, shown: a MissionVerifyShot, or (pair) two of them, before
  // and after, side by side.
  children: ReactNode;
  pair?: boolean;
  // What it shows.
  caption?: ReactNode;
  // The command that made it.
  madeBy?: string;
};

// MissionVerifyEvidence is a file that shows a check, as it is, with what
// it shows and the command that made it under it.
export function MissionVerifyEvidence({ children, pair, caption, madeBy }: MissionVerifyEvidenceProps) {
  return (
    <figure className={css.evidence}>
      {pair ? <div className={css.pair}>{children}</div> : children}
      {(caption || madeBy) && (
        <figcaption>
          {caption}
          {madeBy && (
            <span>
              Made by <MissionVerifyCommand text={madeBy} />
            </span>
          )}
        </figcaption>
      )}
    </figure>
  );
}
