import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { ModelInfo } from "../api";
import samples from "./modelSamples.json";
import { ModelSwitch } from "./ModelSwitch";

// What a conversation or running crew is on, in its head or beside Send:
// switched from the same panel as a role's model. On Cursor's list; one
// whose model isn't set, on its harness's default; one working in a
// terminal, which can't be switched till its turn ends.

const meta: Meta = { title: "Models/ModelSwitch" };
export default meta;

const cursor = samples.cursor as ModelInfo[];

function Switch({ value: start, unavailable }: { value: string; unavailable?: string }) {
  const [value, setValue] = useState(start);
  const [busy, setBusy] = useState(false);
  return (
    <div style={{ padding: 32, minHeight: 640, color: "var(--dim)", fontSize: "var(--text-sm)" }}>
      c2 · cursor{" "}
      <ModelSwitch
        models={cursor}
        value={value}
        placeholder="cursor's default"
        busy={busy}
        unavailable={unavailable}
        aria-label="c2's model"
        onSwitch={(m) => {
          setBusy(true);
          setTimeout(() => {
            setValue(m);
            setBusy(false);
          }, 600);
        }}
      />
    </div>
  );
}

export const On: StoryObj = { render: () => <Switch value="claude-opus-5-5-high" /> };
export const Default: StoryObj = { render: () => <Switch value="" /> };
export const Working: StoryObj = { render: () => <Switch value="claude-sonnet-5-5-medium" unavailable="c2 is working: its model is typed into its terminal, so switch it once its turn ends" /> };
