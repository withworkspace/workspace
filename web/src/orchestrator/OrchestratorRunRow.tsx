import type { MouseEventHandler } from "react";
import { Link } from "react-router";
import { Chip } from "../ui/Chip";
import { Inline } from "../ui/Inline";
import { Row, RowLine, RowMain, RowSub, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";
import { Time } from "../ui/Time";
import css from "./OrchestratorRunRow.module.css";
import { Text } from "../ui/Text";

type OrchestratorRunRowProps = {
  to: string;
  // How it went: its icon, and its words in a chip.
  outcome: StatusInfo;
  // The mission it was of, where runs of several are listed.
  mission?: string;
  // What it looked at ("c1 (done)").
  on: string;
  // The start of what it said.
  said?: string;
  started: string;
  onContextMenu?: MouseEventHandler;
};

// OrchestratorRunRow is a run of the orchestrator's in brief, a link to
// its page: how it went, what it looked at, the start of what it said,
// and when.
export function OrchestratorRunRow({ to, outcome, mission, on, said, started, onContextMenu }: OrchestratorRunRowProps) {
  return (
    <Row>
      <Link to={to} className={rowLinkClass(css.link)} onContextMenu={onContextMenu}>
        <Status s={outcome} iconOnly size={16} className={rowStatusClass()} />
        <RowMain>
          <RowLine>
            {mission && <RowTitle>{mission}</RowTitle>}
            <Text dim>on {on}</Text>
            <Chip tone={outcome.tone}>{outcome.words}</Chip>
          </RowLine>
          {said && (
            <RowSub>
              <Inline text={said} lines={1} />
            </RowSub>
          )}
        </RowMain>
        <Time iso={started} className={css.time} />
      </Link>
    </Row>
  );
}
