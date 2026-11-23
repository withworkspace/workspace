import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Switch } from "./Switch";

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
      <span className="dim">Off</span>
      <One />
      <span className="dim">On</span>
      <One start />
      <span className="dim">Disabled</span>
      <One disabled />
      <span className="dim">Disabled, on</span>
      <One start disabled />
      <span className="dim">In a row</span>
      <label style={{ display: "flex", alignItems: "center", gap: 12, width: 280, justifyContent: "space-between" }}>
        Tell the desktop when something needs you <One start />
      </label>
    </div>
  ),
};
