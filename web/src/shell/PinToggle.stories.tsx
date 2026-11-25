import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { PinToggle } from "./PinToggle";
import { Text } from "../ui/Text";

// Pin, side by side: off and on, as the pin alone (a head with no room
// for its words), and kept to the end of its line, as a ref card's foot
// and an artifact's title line have it. Each pins and unpins when
// pressed.

const meta: Meta = { title: "Shell/PinToggle" };
export default meta;

function Pin({ start, compact, end, keys }: { start: boolean; compact?: boolean; end?: boolean; keys?: string }) {
  const [on, setOn] = useState(start);
  return <PinToggle on={on} compact={compact} end={end} keys={keys} onToggle={() => setOn(!on)} />;
}

const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8 };

export const States: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, maxWidth: 420 }}>
      <div style={row}>
        <Pin start={false} />
        <Pin start />
        <Pin start={false} keys="." />
      </div>
      <div style={row}>
        <Pin start={false} compact />
        <Pin start compact />
      </div>
      <div style={{ ...row, borderTop: "1px solid var(--line)", paddingTop: 8 }}>
        <Text dim>⌘-click its words to go there</Text>
        <Pin start={false} end />
      </div>
      <div style={{ ...row, borderTop: "1px solid var(--line)", paddingTop: 8 }}>
        <span>Invoice totals off by a penny</span>
        <Pin start end />
      </div>
    </div>
  ),
};
