import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Icon } from "./Icon";
import { Popover } from "./Popover";
import { Segment, Segmented } from "./Segmented";
import { Switch } from "./Switch";

// A popover: a button, and what it opens beside it. Press each to see
// where it opens: under the button at its start, at its end, above it.

const meta: Meta = { title: "Components/Popover" };
export default meta;

function Controls() {
  const [width, setWidth] = useState("fit");
  const [grid, setGrid] = useState(true);
  return (
    <div style={{ display: "grid", gap: 12, padding: 12, width: 260 }}>
      <label style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
        Show the grid <Switch on={grid} onChange={setGrid} aria-label="Show the grid" />
      </label>
      <Segmented fill value={width} onChange={setWidth} aria-label="Width">
        <Segment value="fit">Fit</Segment>
        <Segment value="768">768</Segment>
        <Segment value="390">390</Segment>
      </Segmented>
    </div>
  );
}

const trigger = (words: string) => (
  <Button variant="ghost" size="small">
    <Icon name="sliders" size={13} /> {words}
  </Button>
);

export const Placements: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 24, justifyContent: "center", paddingTop: 160 }}>
      <Popover trigger={trigger("Under, at its start")} aria-label="Tweaks">
        <Controls />
      </Popover>
      <Popover trigger={trigger("Under, at its end")} placement="bottom end" aria-label="Tweaks">
        <Controls />
      </Popover>
      <Popover trigger={trigger("Above")} placement="top" aria-label="Tweaks">
        <Controls />
      </Popover>
    </div>
  ),
};

// Words alone, for more about something.
export const Words: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", justifyContent: "center", paddingTop: 40 }}>
      <Popover trigger={<Button>What's a capsule?</Button>} placement="bottom" aria-label="What a capsule is">
        <p style={{ margin: 0, padding: 12, width: 280 }}>A capsule is a checkout of a repo for one piece of work: its own folder and branch, lifted from the ground and burned when it's landed.</p>
      </Popover>
    </div>
  ),
};
