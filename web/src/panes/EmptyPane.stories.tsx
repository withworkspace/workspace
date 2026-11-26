import type { Meta, StoryObj } from "@storybook/react-vite";
import { EmptyPane } from "./EmptyPane";

// Beside docked, on a page it has nothing for: it stays, so the page
// keeps its width as you go, and says how something comes to be in it.

const meta: Meta = { title: "Chrome/EmptyPane" };
export default meta;

const dock: React.CSSProperties = { width: 420, height: 480, border: "1px solid var(--line)", borderRadius: 10, background: "var(--bg)" };

export const Nothing: StoryObj = {
  render: () => (
    <div style={dock}>
      <EmptyPane />
    </div>
  ),
};

export const OpenElsewhere: StoryObj = {
  render: () => (
    <div style={dock}>
      <EmptyPane elsewhere={2} />
    </div>
  ),
};
