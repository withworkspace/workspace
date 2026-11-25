import type { ReactNode } from "react";
import { Chip } from "../ui/Chip";
import { Copyable } from "../ui/Copyable";
import css from "./ArtifactTweaksHead.module.css";

type ArtifactTweaksHeadProps = {
  // How many tweaks aren't as the page opens them.
  changed: number;
  // What each is set to, as text: what Copy copies.
  copy: string;
  // What's pressed at its end, after Copy: the button that hides the tray.
  children?: ReactNode;
};

// ArtifactTweaksHead is the head over a frame's tweaks.
export function ArtifactTweaksHead({ changed, copy, children }: ArtifactTweaksHeadProps) {
  return (
    <header className={css.head}>
      <h2>Tweaks</h2>
      {changed ? <Chip>{changed} changed</Chip> : null}
      <span className={css.end}>
        <Copyable text={copy} className={css.copy} title="Copy what each is set to, as text: to say which you'd keep">
          Copy
        </Copyable>
        {children}
      </span>
    </header>
  );
}
