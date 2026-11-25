import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { endedCapsules, zChecked } from "../samples";
import { EndedCapsules as Ended } from "./EndedCapsules";
import { MissionEndingDialog } from "./MissionEndingDialog";
import { MissionEndingHandedOn } from "./MissionEndingHandedOn";
import { ZChecks as Z } from "./ZChecks";

// How a mission ends: Z as it was checked (two missions ws gave, and a
// Z checked part by part: met, not met, nobody checked), where it was
// handed on to, capsules that ended alone, and the dialog that ends one.

const meta: Meta = { title: "Mission/MissionEnding" };
export default meta;

const none = () => {};
const col = (children: ReactNode, key?: string) => (
  <div key={key} style={{ width: 420, flex: "none" }}>
    {children}
  </div>
);
const taken = (name: string) => takenMissions.find((m) => m.name === name)!.d;

export const ZChecks: StoryObj = {
  render: () => {
    storyWorkspace();
    const ended = taken("which-index-is-missing").ending!;
    const conclude = taken("which-cache-is-slow").conclude!;
    return (
      <RefsProvider>
        <div style={{ display: "flex", gap: 24, alignItems: "start", padding: 16, background: "var(--bg)" }}>
          {col(<><Z c={ended} /><MissionEndingHandedOn to={ended.to} /></>, "a")}
          {col(<><Z c={conclude} /><MissionEndingHandedOn to={conclude.to} /></>, "b")}
          {col(<><Z c={zChecked} /><MissionEndingHandedOn to={zChecked.to} /></>, "c")}
        </div>
      </RefsProvider>
    );
  },
};

export const EndedCapsules: StoryObj = {
  render: () => <div style={{ width: 360, padding: 16, background: "var(--bg)" }}><Ended list={endedCapsules} /></div>,
};

// A capsule's own endings, as its card in the Work tab offers them, with
// the dialog of one open.
const ends = (dialog: ReactNode) => (
  <div style={{ padding: 16, background: "var(--bg)" }}>
    <Button variant="ghost" size="small" onClick={none} title="Hand it on to a mission of its own, which takes it">
      <Icon name="arrow-right" size={12} /> Hand on
    </Button>
    <Button variant="ghost" size="small" onClick={none} title="Drop it: its work won't be part of the mission">
      <Icon name="x" size={12} /> Drop
    </Button>
    {dialog}
  </div>
);
const name = () => {
  const c = taken("work-across-repos").capsules[0];
  return `${c.repo}/${c.name}`;
};

export const HandOn: StoryObj = {
  render: () => ends(<MissionEndingDialog ending="hand-on" name={name()} text="Greet in the reader's language" onText={none} to="greet-by-name" onTo={none} onClose={none} onEnd={none} />),
};

export const Drop: StoryObj = {
  render: () => ends(<MissionEndingDialog ending="drop" name={name()} text="" onText={none} to="" onTo={none} error="This is a story: nothing's written from here." onClose={none} onEnd={none} />),
};
