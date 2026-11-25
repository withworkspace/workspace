import type { Meta, StoryObj } from "@storybook/react-vite";
import { DirtyDot } from "./DirtyDot";
import { RepoDot } from "./RepoDot";

// The dots beside a name: a repo's colour (RepoDot), and changes not
// committed (DirtyDot), each beside the words they're said with.

const meta: Meta = { title: "Components/Dots" };
export default meta;

const line = { display: "flex", gap: 8, alignItems: "center" } as const;

export const Dots: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 12 }}>
      {["#3b6fd4", "#b4532a", "#2f9e6e", "#e8e8ec"].map((c) => (
        <div key={c} style={line}>
          <RepoDot color={c} /> a repo in {c}
        </div>
      ))}
      <div style={line}>
        src/report/summary.ts <DirtyDot />
      </div>
      <div style={line}>
        fx <DirtyDot title="Changes not committed" />
      </div>
    </div>
  ),
};
