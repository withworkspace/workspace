import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Option, Select } from "./Select";
import { Input } from "./Input";
import { Text } from "./Text";

// Selects: the size of a text box, small, tiny (in a line of words),
// filling what it's in, disabled, and with a note on a choice.

const meta: Meta = { title: "Components/Select" };
export default meta;

const row = { display: "flex", gap: 16, alignItems: "center", flexWrap: "wrap" } as const;

function Agents(props: { size?: "small" | "tiny"; fill?: boolean; disabled?: boolean }) {
  const [agent, setAgent] = useState("claude");
  return (
    <Select {...props} value={agent} onChange={setAgent} aria-label="Agent">
      <Option value="claude">claude</Option>
      <Option value="codex">codex</Option>
      <Option value="cursor">cursor</Option>
      <Option value="opencode" note="not here">
        opencode
      </Option>
      <Option value="long" title="An agent whose name is long enough that it's cut short where the list ends">
        An agent whose name is long enough that it's cut short where the list ends
      </Option>
    </Select>
  );
}

export const Sizes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 20 }}>
      <div style={row}>
        <Text dim style={{ width: 80 }}>
          A text box's
        </Text>
        <Agents />
        <Input type="text" defaultValue="A text box beside it" style={{ width: 200 }} />
      </div>
      <div style={row}>
        <Text dim style={{ width: 80 }}>
          Small
        </Text>
        <Agents size="small" />
      </div>
      <div style={row}>
        <Text dim style={{ width: 80 }}>
          Tiny
        </Text>
        <Text dim>
          Sorted by <Agents size="tiny" />
        </Text>
      </div>
      <div style={row}>
        <Text dim style={{ width: 80 }}>
          Fill
        </Text>
        <div style={{ width: 360 }}>
          <Agents fill />
        </div>
      </div>
      <div style={row}>
        <Text dim style={{ width: 80 }}>
          Disabled
        </Text>
        <Agents disabled />
      </div>
    </div>
  ),
};
