import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { githubRepos, none } from "./samples";
import { SettingsAddRepoAll } from "./SettingsAddRepoAll";
import { SettingsAddRepoBar } from "./SettingsAddRepoBar";
import { SettingsAddRepoForm } from "./SettingsAddRepoForm";
import { SettingsAddRepoList } from "./SettingsAddRepoList";
import { SettingsAddRepoOption } from "./SettingsAddRepoOption";
import { SettingsGroupPick } from "./SettingsGroupPick";

// Add a repo, as its dialog holds it: whose and the filter over an
// owner's repos (one that's here already can't be picked), forks and
// archived ones behind their box, and the one chosen about to be added;
// another owner being named while GitHub's asked; a list with nothing in
// it; and a name that's taken.

const meta: Meta = { title: "Settings/SettingsAddRepo" };
export default meta;

const owners = [{ login: "you", you: true }, { login: "sorrel" }];

function Picking() {
  const [owner, setOwner] = useState("sorrel");
  const [another, setAnother] = useState(false);
  const [named, setNamed] = useState("");
  const [q, setQ] = useState("");
  const [all, setAll] = useState(false);
  const [chosen, setChosen] = useState<string | undefined>("ledger");
  const [as, setAs] = useState("ledger");
  const [team, setTeam] = useState(false);
  const [group, setGroup] = useState("");
  const shown = githubRepos.filter((r) => (all || (!r.fork && !r.archived)) && r.name.includes(q));
  const bad = !/^[A-Za-z0-9_-][A-Za-z0-9._-]*$/.test(as.trim());
  return (
    <>
      <SettingsAddRepoBar owners={owners} owner={owner} another={another} naming={another} named={named} filter={q} when="as of 4 minutes ago" onOwner={(o) => (setAnother(false), setOwner(o))} onAnother={() => setAnother(true)} onNamed={setNamed} onFilter={setQ} onLook={none} />
      <SettingsAddRepoList label="sorrel's repos" value={chosen} onChange={setChosen} none={q ? "None of these match." : "Nothing to add here."}>
        {shown.map((r) => (
          <SettingsAddRepoOption key={r.name} repo={r} />
        ))}
      </SettingsAddRepoList>
      <SettingsAddRepoAll hidden={2} checked={all} onChange={setAll} />
      {chosen && (
        <SettingsAddRepoForm
          as={as}
          invalid={bad}
          hint={bad ? "Letters, digits, '.', '_' and '-'." : as.trim() !== chosen ? `It stays sorrel/${chosen} on GitHub.` : "Its folder under repos/, and what commands call it."}
          onAs={setAs}
          group={<SettingsGroupPick value={group} groups={["product", "apps"]} none label="Its group" onPick={setGroup} />}
          team={team}
          onTeam={setTeam}
          add={team ? `Add sorrel/${chosen} for the team` : `Add sorrel/${chosen}`}
          onCancel={() => setChosen(undefined)}
          onSubmit={none}
        />
      )}
    </>
  );
}

const box = (children: React.ReactNode) => (
  <div style={{ width: 640, maxWidth: "100%", margin: "32px auto", padding: 20, borderRadius: 12, background: "var(--raised)", boxShadow: "0 0 0 1px var(--line)" }}>
    {children}
  </div>
);

export const Picking_: StoryObj = { name: "Picking", render: () => box(<Picking />) };

export const AskingGitHub: StoryObj = {
  name: "Another owner, while GitHub's asked",
  render: () =>
    box(
      <>
        <SettingsAddRepoBar owners={owners} owner="withworkspace" another naming named="withworkspace" filter="" when="asking GitHub…" looking onOwner={none} onAnother={none} onNamed={none} onFilter={none} onLook={none} />
        <SettingsAddRepoList label="withworkspace's repos" onChange={none} loading>
          {null}
        </SettingsAddRepoList>
      </>,
    ),
};

export const NothingToAdd: StoryObj = {
  name: "Nothing to add, and a name that's taken",
  render: () =>
    box(
      <>
        <SettingsAddRepoBar owners={[]} owner="" another={false} naming named="" filter="" when="" onOwner={none} onAnother={none} onNamed={none} onFilter={none} onLook={none} />
        <SettingsAddRepoList label="repos" onChange={none} none="Nothing to add here.">
          {null}
        </SettingsAddRepoList>
        <SettingsAddRepoForm as="api" invalid hint="There's a api here already." onAs={none} group={<SettingsGroupPick value="" groups={[]} none label="Its group" onPick={none} />} team onTeam={none} add="Add sorrel/api for the team" onCancel={none} onSubmit={none} />
      </>,
    ),
};
