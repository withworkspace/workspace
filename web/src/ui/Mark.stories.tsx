import type { Meta, StoryObj } from "@storybook/react-vite";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { Mark } from "./Mark";

// A mark beside the two things it mustn't be mistaken for: a count of what
// needs you (amber, a number), and nothing at all.

const meta: Meta = { title: "Library/Mark" };
export default meta;

const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, width: 200 };

export const Marks: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 10 }}>
      <div style={row}>
        <Icon name="file" size={15} />
        <span style={{ flex: 1 }}>Documents</span>
        <Mark title="ines amended ADR 0003" />
      </div>
      <div style={row}>
        <Icon name="flag" size={15} />
        <span style={{ flex: 1 }}>Missions</span>
        <Count tone="attention" title="4 need you">
          4
        </Count>
      </div>
      <div style={row}>
        <Icon name="user" size={15} />
        <span style={{ flex: 1 }}>Team</span>
      </div>
      <div style={row}>
        <span style={{ position: "relative", display: "inline-grid" }}>
          <Icon name="bell" size={15} />
          <Mark ringed title="Something new" className="story-corner" />
        </span>
        <span style={{ flex: 1 }}>Ringed, over an icon's corner</span>
      </div>
      <div style={row}>
        <Mark tone="danger" title="A check failed" />
        <span style={{ flex: 1 }}>Something failing</span>
      </div>
      <style>{`.story-corner { position: absolute; top: -1px; right: -2px; }`}</style>
    </div>
  ),
};
