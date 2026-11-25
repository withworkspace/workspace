import type { MouseEventHandler } from "react";
import { Link } from "react-router";
import { Inline } from "../ui/Inline";
import { Row, RowTitle, rowLinkClass } from "../ui/Rows";
import { Status, type StatusInfo } from "../ui/Status";

type OrchestratorMissionRowProps = {
  to: string;
  title: string;
  // Where the mission stands, and its headline's words.
  status: StatusInfo;
  headline: string;
  onContextMenu?: MouseEventHandler;
};

// OrchestratorMissionRow is a mission the orchestrator's deciding on now,
// a link to it.
export function OrchestratorMissionRow({ to, title, status, headline, onContextMenu }: OrchestratorMissionRowProps) {
  return (
    <Row>
      <Link to={to} className={rowLinkClass()} onContextMenu={onContextMenu}>
        <Status s={status} iconOnly />
        <RowTitle>{title}</RowTitle>
        <span className="dim">
          <Inline text={headline} lines={1} />
        </span>
      </Link>
    </Row>
  );
}
