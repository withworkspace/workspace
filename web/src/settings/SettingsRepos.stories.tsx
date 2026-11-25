import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { none, repos } from "./samples";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsGroupPick } from "./SettingsGroupPick";
import { SettingsGroupRow } from "./SettingsGroupRow";
import { SettingsGroups } from "./SettingsGroups";
import { SettingsRepoGroup } from "./SettingsRepoGroup";
import { SettingsRepoRow } from "./SettingsRepoRow";
import { SettingsRepoShare } from "./SettingsRepoShare";
import { SettingsReposTable } from "./SettingsReposTable";
import { SettingsReposTitle } from "./SettingsReposTitle";

// Settings' Repos: the table, with a repo as the team has it, one whose
// group is yours over the team's, one that lands straight on its branch,
// one being cloned, one that couldn't be, and one of yours (to give to
// the team: pressed, it asks first); under it the groups, one hidden by
// you over the team's; and what's said where there are no groups yet. In a window 640px wide or
// less a table's fourth column goes.

const meta: Meta = { title: "Settings/SettingsRepos" };
export default meta;

const groups = ["product", "apps"];
const labels = new Map([
  ["product", "Product"],
  ["apps", "Apps"],
]);

function Group({ value, team, over }: { value: string; team?: string; over?: string }) {
  const [group, setGroup] = useState(value);
  return (
    <SettingsRepoGroup over={over}>
      <SettingsGroupPick value={group} groups={groups} labels={labels} team={team} none={!team} label="Its group" onPick={setGroup} />
    </SettingsRepoGroup>
  );
}

function Share({ repo }: { repo: string }) {
  const [asking, setAsking] = useState(false);
  return <SettingsRepoShare repo={repo} asking={asking} onAsk={() => setAsking(true)} onShare={none} onCancel={() => setAsking(false)} />;
}

function Hidden({ name, label, list, hidden: was, chosen, teamHidden }: { name: string; label?: string; list: string; hidden?: boolean; chosen?: boolean; teamHidden?: boolean }) {
  const [hidden, setHidden] = useState(was);
  return <SettingsGroupRow name={name} label={label} repos={list} hidden={hidden} chosen={chosen} teamHidden={teamHidden} onSet={setHidden} onReset={none} />;
}

function Repos({ acts = true }: { acts?: boolean }) {
  const shown = acts ? repos : repos.filter((r) => !r.yours);
  return (
    <SettingsBlock
      id="repos"
      title={<SettingsReposTitle count={shown.length} onAdd={none} />}
      about={
        <>
          The team's are in <code>ws.toml</code>: shown here, changed there.
        </>
      }
    >
      <SettingsReposTable acts={acts}>
        {shown.map((r) => (
          <SettingsRepoRow
            key={r.name}
            repo={r}
            lands={r.landing === "direct" ? "straight to main" : "pull request"}
            group={<Group value={r.group ?? ""} team={r.yours ? undefined : r.groupYours ? r.teamGroup : r.group} over={r.groupYours ? `The team has it in ${r.teamGroup}` : undefined} />}
            acts={acts}
          >
            {r.yours && <Share repo={r.name} />}
          </SettingsRepoRow>
        ))}
      </SettingsReposTable>
      <SettingsGroups>
        <Hidden name="product" label="Product" list="API, Web" />
        <Hidden name="apps" label="Apps" list="—" hidden chosen teamHidden={false} />
        <Hidden name="infra" list="Billing, Legacy" />
      </SettingsGroups>
    </SettingsBlock>
  );
}

export const Table: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <Repos />
    </div>
  ),
};

export const TheTeamsAlone: StoryObj = {
  name: "The team's alone, with no groups",
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <SettingsBlock title={<SettingsReposTitle count={2} onAdd={none} />}>
        <SettingsReposTable>
          {repos.slice(0, 2).map((r) => (
            <SettingsRepoRow key={r.name} repo={{ ...r, group: undefined, groupYours: false }} lands="pull request" group={<Group value="" />} />
          ))}
        </SettingsReposTable>
        <SettingsGroups none />
      </SettingsBlock>
    </div>
  ),
};
