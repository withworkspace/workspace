import type { ReactNode } from "react";
import { Link, type To } from "react-router";
import { buttonClass } from "../../ui/Button";
import css from "./MissionWorkCrewLine.module.css";

type MissionWorkCrewLineProps = {
  badge: ReactNode;
  // Where it stands or how it ended, the steps it took and when.
  children: ReactNode;
  // To its terminal, or its transcript once it's ended.
  to: To;
  link: ReactNode;
  linkTitle?: string;
  // What it left, under its line: a MissionWorkLeft.
  left?: ReactNode;
};

// MissionWorkCrewLine is a crew who's been in the capsule that's open, in
// a line, with what it left under it.
export function MissionWorkCrewLine({ badge, children, to, link, linkTitle, left }: MissionWorkCrewLineProps) {
  return (
    <div>
      <div className={css.line}>
        {badge}
        <span className={css.say}>{children}</span>
        <Link to={to} className={buttonClass({ size: "small" }, css.link)} title={linkTitle}>
          {link}
        </Link>
      </div>
      {left}
    </div>
  );
}
