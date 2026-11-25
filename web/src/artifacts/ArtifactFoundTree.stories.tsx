import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import { ArtifactFoundFolder, ArtifactFoundTree } from "./ArtifactFoundTree";
import { ArtifactRow } from "./ArtifactRow";
import { ArtifactsFound } from "./ArtifactsFound";
import { days } from "./samples";

// The artifacts ws found in the workspace's repos, from their parts: as a
// tree (a repo, the folders its artifacts are grouped in, and theirs, one
// folded), and as a plain list, where one repo has them all in one place.

const meta: Meta = { title: "Artifacts/Found" };
export default meta;

function Found({ title, where, by, at }: { title: string; where: string; by: string; at: string }) {
  return (
    <ArtifactRow to={`/artifacts/in/${where}`} title={title} chips={<Chip tone="done">Promoted</Chip>} when={{ by, at, title: by }}>
      <span title={`It lives in records:${where}`}>
        <Icon name="branch" size={11} /> Records
      </span>
      <span className="mono">records:{where}</span>
    </ArtifactRow>
  );
}

function Folder({ name, count, title, folded, rows, children }: { name: string; count: number; title: string; folded?: boolean; rows?: ReactNode; children?: ReactNode }) {
  const [open, setOpen] = useState(!folded);
  return (
    <ArtifactFoundFolder name={name} count={count} title={title} open={open} onToggle={setOpen} rows={rows}>
      {children}
    </ArtifactFoundFolder>
  );
}

const glow = <Found title="Horizon glow" where="artifacts/2026-11-20-horizon-glow/" by="v2 promoted by Priya" at={days(5)} />;
const kanban = <Found title="Kanban columns" where="artifacts/boards/2026-11-02-kanban/" by="v1 promoted by you" at={days(23)} />;
const lanes = <Found title="Swimlanes" where="artifacts/boards/2026-10-30-swimlanes/" by="changed by Tomás" at={days(26)} />;
const first = <Found title="The first board" where="artifacts/boards/archive/2026-09-12-first-board/" by="" at={days(74)} />;

export const Tree: StoryObj = {
  render: () => (
    <div className="page narrow">
      <ArtifactsFound>
        <ArtifactFoundTree>
          <Folder name="Records" count={4} title="records" rows={glow}>
            <Folder
              name="boards"
              count={3}
              title="records: boards/"
              rows={
                <>
                  {lanes}
                  {kanban}
                </>
              }
            >
              <Folder name="archive" count={1} title="records: boards/archive/" rows={first} />
              <Folder name="drafts" count={2} title="records: boards/drafts/" folded rows={first} />
            </Folder>
          </Folder>
          <Folder name="Web" count={1} title="web" rows={<Found title="Dashboard, dark" where="design/2026-11-18-dashboard-dark/" by="v3 promoted by you" at={days(7)} />} />
        </ArtifactFoundTree>
      </ArtifactsFound>
    </div>
  ),
};

export const PlainList: StoryObj = {
  render: () => (
    <div className="page narrow">
      <ArtifactsFound>
        <Rows>
          {glow}
          {kanban}
        </Rows>
      </ArtifactsFound>
    </div>
  ),
};
