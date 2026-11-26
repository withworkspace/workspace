import type { Meta, StoryObj } from "@storybook/react-vite";
import { Logotype } from "./Logotype";
import { Text } from "./Text";

// The logotype: at the size the workspace's menu has it, at What's new's,
// and large; and beside words, where it sits on their line.

const meta: Meta = { title: "Components/Logotype" };
export default meta;

export const Sizes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 20, justifyItems: "start" }}>
      <Logotype />
      <Logotype height={22} />
      <Logotype height={48} />
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <Logotype />
        <Text dim mono>v0.15.0</Text>
      </div>
    </div>
  ),
};
