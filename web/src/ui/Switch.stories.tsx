import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Switch } from "./Switch";
import { Text } from "./Text";

// A switch: off, on, and disabled either way. Press one, or focus it and
// press Space.

const meta: Meta = { title: "Components/Switch" };
export default meta;

function One({ start = false, disabled }: { start?: boolean; disabled?: boolean }) {
  const [on, setOn] = useState(start);
  return <Switch on={on} onChange={setOn} disabled={disabled} aria-label="Tell the desktop" />;
}

export const States: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 12, gridTemplateColumns: "100px auto", justifyContent: "start", alignItems: "center" }}>
      <Text dim>Off</Text>
      <One />
      <Text dim>On</Text>
      <One start />
      <Text dim>Disabled</Text>
      <One disabled />
      <Text dim>Disabled, on</Text>
      <One start disabled />
      <Text dim>In a row</Text>
      <label style={{ display: "flex", alignItems: "center", gap: 12, width: 280, justifyContent: "space-between" }}>
        Tell the desktop when something needs you <One start />
      </label>
    </div>
  ),
};
