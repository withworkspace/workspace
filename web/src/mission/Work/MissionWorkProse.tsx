import type { ReactNode } from "react";
import { cx } from "../../ui/cx";
import css from "./MissionWorkProse.module.css";
import { Prose } from "../../ui/Prose";

// MissionWorkProse is an output's text, to read in the Work tab: prose
// (markdown, drawn), or as it's written (code), at a width that reads.
export function MissionWorkProse({ children, code }: { children: ReactNode; code?: boolean }) {
  return code ? <pre className={cx(css.code, css.prose)}>{children}</pre> : <Prose className={css.prose}>{children}</Prose>;
}
