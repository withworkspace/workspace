import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Empty } from "../../ui/Empty";
import { Keys } from "../../ui/Keys";
import { FilesMission } from "./FilesMission";
import { FilesPick } from "./FilesPick";
import { FilesRepoGroup } from "./FilesRepoGroup";
import { FilesRepoRow } from "./FilesRepoRow";
import { FilesRepos } from "./FilesRepos";
import { FilesReposPage } from "./FilesReposPage";
import { FilesSwitch } from "./FilesSwitch";
import { FilesSwitchItem, FilesSwitchMenu } from "./FilesSwitchMenu";

// The Files page, from its parts: the workspace's repos in their groups
// (one open, one not cloned yet, a hidden group, the spaces of documents
// folded at the foot) beside what's said with none open; the title of a
// repo's worktree, which switches to its ground or a capsule, closed and
// open; and the mission a capsule is for.

const meta: Meta = { title: "Files/Repos" };
export default meta;

const none = () => {};

const column = (
  <FilesRepos count={7}>
    <FilesRepoGroup kind="shown" count={1}>
      <FilesRepoRow to="/" name="tools" label="Tools" color="#8a8f98" capsules={0} cloned />
    </FilesRepoGroup>
    <FilesRepoGroup kind="shown" label="Product" count={3}>
      <FilesRepoRow to="/" name="api" label="API" color="#3b6fd4" capsules={8} on cloned />
      <FilesRepoRow to="/" name="web" label="Web" color="#b4532a" capsules={2} cloned />
      <FilesRepoRow to="/" name="mobile" label="Mobile" color="#2f9e6e" capsules={0} />
    </FilesRepoGroup>
    <FilesRepoGroup kind="hidden" label="Archive" count={1}>
      <FilesRepoRow to="/" name="old-site" label="Old site" color="#9b59b6" capsules={0} cloned />
    </FilesRepoGroup>
    <FilesRepoGroup kind="hidden" label="Infra" count={1} open>
      <FilesRepoRow to="/" name="infra" label="Infra" color="#e6a23c" capsules={1} cloned />
    </FilesRepoGroup>
    <FilesRepoGroup kind="documents" count={1}>
      <FilesRepoRow to="/" name="records" label="Records" color="#5c6b7a" capsules={0} cloned />
    </FilesRepoGroup>
  </FilesRepos>
);

export const NoneOpen: StoryObj = {
  render: () => (
    <div style={{ height: 560 }}>
      <FilesReposPage column={column}>
        <FilesPick>
          <Empty icon="layers" title="Pick a repo">
            Its files as they are on its default branch (the ground), or in any of its capsules: its title switches between them (<Keys k="w" />).
          </Empty>
        </FilesPick>
      </FilesReposPage>
    </div>
  ),
};

export const Narrow: StoryObj = {
  render: () => (
    <div style={{ height: 560, width: 390 }}>
      <FilesReposPage narrow column={column} />
    </div>
  ),
};

function Switch({ open: start, ground, capsules = true }: { open?: boolean; ground?: boolean; capsules?: boolean }) {
  const [open, setOpen] = useState(!!start);
  const [on, setOn] = useState(1);
  const items = [
    { name: ".ground", ground: true, sub: "main, the default branch", behind: 2 },
    { name: "pdf-vat-rows", sub: "Show VAT by rate on the PDF", ahead: 3, dirty: true },
    { name: "tax-rounding", sub: "Invoice totals off by a penny" },
    { name: "spike", sub: "spike-branch" },
  ];
  const shown = capsules ? items : items.slice(0, 1);
  const current = ground ? ".ground" : "pdf-vat-rows";
  return (
    <div style={{ padding: "16px 24px 360px" }}>
      <FilesSwitch
        repo="API"
        color="#3b6fd4"
        name={current}
        ground={ground}
        dirty={!ground}
        open={open}
        onToggle={() => setOpen(!open)}
        menu={
          <FilesSwitchMenu label="API's ground and capsules" none={capsules ? undefined : <>No capsules yet: ws lift api &lt;name&gt; makes one.</>}>
            {shown.map((w, i) => (
              <FilesSwitchItem key={w.name} {...w} current={w.name === current} on={i === on} onHover={() => setOn(i)} onPick={none} />
            ))}
          </FilesSwitchMenu>
        }
      />
    </div>
  );
}

export const SwitchClosed: StoryObj = { render: () => <Switch /> };
export const SwitchOpen: StoryObj = { render: () => <Switch open /> };
export const SwitchOnTheGround: StoryObj = { render: () => <Switch open ground /> };
export const SwitchWithNoCapsules: StoryObj = { render: () => <Switch open ground capsules={false} /> };

export const Mission: StoryObj = {
  render: () => (
    <div style={{ display: "flex", gap: 12, padding: 24 }}>
      <FilesMission to="/" title="Show VAT by rate on the PDF" />
      <FilesMission to="/" title="A mission whose title goes on for far longer than there's room for in the bar" />
    </div>
  ),
};
