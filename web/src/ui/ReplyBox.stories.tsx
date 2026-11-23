import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { ReplyBox } from "./ReplyBox";

// The reply box: as it's drawn (with the keys, nothing to send yet), and
// one whose words and button say something else. What's sent is shown
// under it.

const meta: Meta = { title: "Components/ReplyBox" };
export default meta;

function One(props: { placeholder?: string; send?: string }) {
  const [said, setSaid] = useState<string>();
  return (
    <div>
      <ReplyBox {...props} onSend={setSaid} onCancel={() => setSaid(undefined)} />
      {said && <div className="dim">Sent: {said}</div>}
    </div>
  );
}

export const Boxes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, maxWidth: 520 }}>
      <One />
      <One placeholder="What was decided, in a line…" send="Record" />
    </div>
  ),
};
