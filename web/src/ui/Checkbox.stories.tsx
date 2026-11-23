import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Checkbox } from "./Checkbox";

// Checkboxes: off, ticked, disabled either way, with more said under the
// words, and one with no words (named by aria-label).

const meta: Meta = { title: "Components/Checkbox" };
export default meta;

function One({ start = false, disabled, children, label }: { start?: boolean; disabled?: boolean; children?: React.ReactNode; label?: string }) {
  const [on, setOn] = useState(start);
  return (
    <Checkbox checked={on} onChange={setOn} disabled={disabled} aria-label={label}>
      {children}
    </Checkbox>
  );
}

export const States: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 12, justifyItems: "start" }}>
      <One>Open it as a draft</One>
      <One start>Read the files in this order</One>
      <One disabled>Disabled</One>
      <One start disabled>
        Disabled, ticked
      </One>
      <One start>
        <span>
          Remove what it kept for itself too
          <span className="dim" style={{ display: "block", fontSize: "var(--text-sm)" }}>
            Left alone, adding it again picks up where it was.
          </span>
        </span>
      </One>
      <span style={{ display: "inline-flex", gap: 8, alignItems: "center" }}>
        <One label="Accept item 3" /> <span className="dim">No words of its own</span>
      </span>
    </div>
  ),
};
