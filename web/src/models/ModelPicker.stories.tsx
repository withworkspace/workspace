import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { ModelInfo } from "../api";
import samples from "./modelSamples.json";
import { ModelPanel } from "./ModelPanel";
import { ModelPicker } from "./ModelPicker";

// A role's model picker, on Cursor's whole list as its CLI gave it
// (2026-11-25): each model once, under its maker, its efforts, thinking
// and fast chosen under the list. Over ACP, Cursor's models have no
// effort or fast to choose (its effort is the session's), so the rows
// are the same and the choices fewer. A harness whose names say nothing
// of where they sit (Claude Code's aliases) is a plain list.

const meta: Meta = { title: "Models/ModelPicker" };
export default meta;

const cursor = samples.cursor as ModelInfo[];
const acp = samples.acp as ModelInfo[];
const claude: ModelInfo[] = [
  { id: "opus", name: "Opus", about: "the latest Opus: the most capable" },
  { id: "sonnet", name: "Sonnet", about: "the latest Sonnet: capable and quicker" },
  { id: "haiku", name: "Haiku", about: "the latest Haiku: small and quick" },
];

function Picker({ models, value: start, open }: { models: ModelInfo[]; value: string; open?: boolean }) {
  const [value, setValue] = useState(start);
  return (
    <div style={{ padding: 32, minHeight: 640 }}>
      <ModelPicker models={models} value={value} onChange={setValue} onAnother={() => {}} placeholder="the agent's own default" aria-label="Model" open={open} />
      <p style={{ marginTop: 12, font: "var(--text-xs) var(--mono)", color: "var(--dim)" }}>{value || "(nothing set)"}</p>
    </div>
  );
}

function Panel({ models, value }: { models: ModelInfo[]; value: string }) {
  const [used, setUsed] = useState(value);
  return (
    <div style={{ padding: 32 }}>
      <div style={{ display: "inline-flex", border: "1px solid var(--line)", borderRadius: 10, background: "var(--raised)", boxShadow: "var(--shadow)" }}>
        <ModelPanel models={models} value={used} onUse={setUsed} onAnother={() => {}} />
      </div>
      <p style={{ marginTop: 12, font: "var(--text-xs) var(--mono)", color: "var(--dim)" }}>{used}</p>
    </div>
  );
}

export const Cursor: StoryObj = { render: () => <Picker models={cursor} value="claude-opus-5-5-high-fast" open /> };
export const CursorPanel: StoryObj = { render: () => <Panel models={cursor} value="claude-opus-5-5-high-fast" /> };
export const ThinkingOnlyAtSome: StoryObj = { render: () => <Panel models={cursor} value="claude-opus-5-thinking-max" /> };
export const CursorACP: StoryObj = { render: () => <Panel models={acp} value="claude-opus-5-thinking" /> };
export const Plain: StoryObj = { render: () => <Panel models={claude} value="sonnet" /> };
export const NothingSet: StoryObj = { render: () => <Picker models={cursor} value="" /> };
export const NotListed: StoryObj = { render: () => <Picker models={cursor} value="claude-opus-9" /> };
