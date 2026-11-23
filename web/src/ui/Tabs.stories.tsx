import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { Tab, Tabs } from "./Tabs";

// Tabs: words alone, with counts, with icons, and what one shows under
// them. Press one, or move along them with the arrows.

const meta: Meta = { title: "Components/Tabs" };
export default meta;

const names = { summary: "Summary", brief: "Brief", work: "Work", verify: "Verify", log: "Log" } as const;
type Name = keyof typeof names;

function Mission() {
  const [tab, setTab] = useState<Name>("summary");
  return (
    <div>
      <Tabs value={tab} onChange={setTab} controls="story-panel" aria-label="Mission">
        {(Object.keys(names) as Name[]).map((t, i) => (
          <Tab key={t} value={t} title={`${names[t]} (${i + 1})`}>
            {names[t]}
            {t === "verify" && <Count tone="attention">2</Count>}
            {t === "work" && <Count>5</Count>}
          </Tab>
        ))}
      </Tabs>
      <div role="tabpanel" id="story-panel" aria-label={names[tab]} className="dim">
        What {names[tab]} shows.
      </div>
    </div>
  );
}

function Crew() {
  const [view, setView] = useState("conversation");
  return (
    <Tabs value={view} onChange={setView} aria-label="Crew">
      <Tab value="conversation">
        <Icon name="message" size={13} /> Conversation
      </Tab>
      <Tab value="terminal">
        <Icon name="terminal" size={13} /> Terminal
      </Tab>
    </Tabs>
  );
}

export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24 }}>
      <Mission />
      <Crew />
      <div style={{ width: 240 }}>
        <span className="dim">Too narrow for the row: it scrolls, and fades at its end.</span>
        <Mission />
      </div>
    </div>
  ),
};
