import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Keys } from "./Keys";
import { Text } from "./Text";

// Keys: one key, a chord with the platform's modifier, one key then
// another, and a key hint after a button's words.

const meta: Meta = { title: "Components/Keys" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <div style={{ display: "flex", gap: 16, alignItems: "baseline" }}>
        <Keys k="c" />
        <Keys k="esc" />
        <Keys k="⏎" />
        <Keys k="mod+K" />
        <Keys k="mod+⏎" />
        <Keys k="mod+shift+P" />
        <Keys k="g m" />
      </div>
      <Text as="div" dim>
        <Keys k="j" /> <Keys k="k" /> to move, <Keys k="⏎" /> to open, <Keys k="?" /> for every key
      </Text>
      <div style={{ display: "flex", gap: 8 }}>
        <Button variant="primary">
          Confirm <Keys k="mod+⏎" />
        </Button>
        <Button>
          Comment <Keys k="c" />
        </Button>
        <Button variant="ghost" size="small">
          Cancel <Keys k="esc" />
        </Button>
      </div>
    </div>
  ),
};
