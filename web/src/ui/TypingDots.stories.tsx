import type { Meta, StoryObj } from "@storybook/react-vite";
import { TypingDots } from "./TypingDots";
import { Text } from "./Text";

// Typing dots: alone, beside the words that say who's at it, and in the
// colours of what they're in.

const meta: Meta = { title: "Components/TypingDots" };
export default meta;

export const Dots: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <TypingDots />
      <Text as="div" dim style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
        <TypingDots /> c2 is working
      </Text>
      <div style={{ display: "inline-flex", alignItems: "center", gap: 8, color: "var(--accent)" }}>
        <TypingDots /> in the accent
      </div>
    </div>
  ),
};
