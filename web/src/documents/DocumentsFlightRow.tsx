import type { ReactNode } from "react";
import { Chip, chipClass } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Row, RowLine, RowMain, RowTitle, rowStatusClass } from "../ui/Rows";
import { TLink } from "../ui/TLink";
import css from "./DocumentsFlightRow.module.css";
import { textClass } from "../ui/Text";

type DocumentsFlightRowProps = {
  capsule: string;
  mission?: string; // the mission it's for
  pr?: { number: number; title: string; to: string }; // the pull request open from it
  space?: ReactNode; // which space, a DocumentsSpaceTag, where there's more than one
  children: ReactNode; // what it changes, each a DocumentsSpaceChange
};

// DocumentsFlightRow is a capsule with changes to documents: its name,
// its mission and pull request, and what it changes.
export function DocumentsFlightRow({ capsule, mission, pr, space, children }: DocumentsFlightRowProps) {
  return (
    <Row still className={css.row}>
      <Icon name="box" size={16} className={rowStatusClass(textClass("dim"))} />
      <RowMain className={css.main}>
        <RowLine>
          <RowTitle className={textClass("mono")}>{capsule}</RowTitle>
          {mission && <Chip tone="quiet">{mission}</Chip>}
          {pr && (
            <TLink to={pr.to} className={chipClass("progress")} title={pr.title}>
              #{pr.number}
            </TLink>
          )}
          {space}
        </RowLine>
        <RowLine className={css.changes}>{children}</RowLine>
      </RowMain>
    </Row>
  );
}
