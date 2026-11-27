import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { SettingsShortcut } from "./SettingsShortcut";
import { SettingsShortcuts, type SettingsShortcutsGroup } from "./SettingsShortcuts";

// Settings › Keyboard's shortcuts: each command's keys, one of yours
// (marked, with Reset), one with no key, one being set, one that clashes
// with another of yours.

const meta: Meta = { title: "Settings/SettingsShortcuts" };
export default meta;

const off = { onAdd: () => {}, onRemove: () => {}, onReset: () => {}, onCancel: () => {} };

const groups: SettingsShortcutsGroup[] = [
  {
    title: "Everywhere",
    shortcuts: [
      { id: "find", title: "Find anything", keys: ["mod+k"], yours: false, ...off },
      { id: "missions", title: "Go to Missions", keys: ["g m"], yours: false, ...off },
      { id: "documents", title: "Go to Documents", keys: ["g d", "g p"], yours: false, ...off },
      { id: "peek", title: "Peek at what's under the pointer; again to close", keys: ["Space"], yours: true, ...off },
    ],
  },
  {
    title: "Team",
    shortcuts: [
      { id: "next", title: "Next", keys: ["j", "ArrowDown"], yours: false, ...off },
      { id: "open", title: "Open: review here, or on GitHub", keys: ["j"], yours: true, clashes: ["j also does Next (Team)"], ...off },
      { id: "summarise", title: "Summarise, or retry", keys: [], yours: true, ...off },
      { id: "refresh", title: "Refresh", keys: ["r"], yours: false, setting: ["g"], ...off },
    ],
  },
];

export const Shortcuts: StoryObj = {
  render: function Render() {
    const [filter, setFilter] = useState("");
    return <SettingsShortcuts groups={groups} filter={filter} onFilter={setFilter} yours={3} onResetAll={() => {}} />;
  },
};

export const NoneFound: StoryObj = {
  render: () => <SettingsShortcuts groups={[]} filter="zzz" onFilter={() => {}} yours={0} onResetAll={() => {}} />,
};

export const Setting: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 720 }}>
      <SettingsShortcut title="Refresh" keys={["r"]} yours={false} setting={[]} {...off} />
      <SettingsShortcut title="Go to Team" keys={["g t"]} yours={false} setting={["g"]} {...off} />
    </div>
  ),
};
