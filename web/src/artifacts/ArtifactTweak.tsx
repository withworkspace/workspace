import type { ReactNode } from "react";
import type { Kind } from "./tweaks";
import css from "./ArtifactTweak.module.css";

// ArtifactTweakList is a page's tweaks, one under another.
export function ArtifactTweakList({ children }: { children: ReactNode }) {
  return <div className={css.list}>{children}</div>;
}

type ArtifactTweakProps = {
  // The id of its control, which its label is for.
  id: string;
  kind: Kind;
  label: string;
  // What it's for, under its label.
  description?: string;
  // How it opens, said, for one that isn't as the page opens it: it's
  // marked, and the mark says so.
  opensAs?: string;
  // inline has its control at the end of its label's line (a switch, a
  // colour): otherwise it's under the label.
  inline?: boolean;
  // What the choice it's at says of itself, under its control.
  chosen?: string;
  // Its control.
  children: ReactNode;
};

// ArtifactTweak is one tweak: its label, what it's for, and its control.
export function ArtifactTweak({ id, kind, label, description, opensAs, inline, chosen, children }: ArtifactTweakProps) {
  return (
    <div className={css.tweak} data-kind={kind}>
      <div className={css.line}>
        <label htmlFor={id} className={css.label}>
          {label}
        </label>
        {opensAs !== undefined && <span className={css.moved} title={`Default: ${opensAs}`} aria-label={`Changed from default: ${opensAs}`} />}
        {inline && <span className={css.inline}>{children}</span>}
      </div>
      {description && <p className={css.description}>{description}</p>}
      {!inline && children}
      {chosen && <p className={css.description}>{chosen}</p>}
    </div>
  );
}
