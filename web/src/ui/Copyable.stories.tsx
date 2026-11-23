import type { Meta, StoryObj } from "@storybook/react-vite";
import { Copyable } from "./Copyable";

// Copyable: a ref, a path and a command that copy themselves; point at
// one for its icon, press it and it says Copied. Then where its icon
// sits: after the words, over their end, or always in the line.

const meta: Meta = { title: "Components/Copyable" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, justifyItems: "start" }}>
      <Copyable text="invoice-totals-off-by-a-penny" />
      <Copyable text="a3.v2" title="Copy its ref" />
      <div className="dim">
        It lives in <Copyable text="records/artifacts/a3" title="Copy its folder" /> too, for everyone with records.
      </div>
      <div>
        Say what it should ask for with <Copyable text="ws plugin config jira --ask">ws plugin config jira --ask</Copyable>.
      </div>
      <Copyable text={"[orchestrator]\nenabled = true"}>Copy</Copyable>
      <div style={{ width: 180, border: "1px dashed var(--line)", padding: 6 }}>
        <Copyable text="a-very-long-slug-that-is-cut-short-where-the-room-ends" icon="over" />
      </div>
      <div>
        web/src/ui/Copyable.tsx{" "}
        <Copyable text="web/src/ui/Copyable.tsx" icon="inline" title="Copy its path">
          {""}
        </Copyable>
      </div>
    </div>
  ),
};
