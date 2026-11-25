import type { Meta, StoryObj } from "@storybook/react-vite";
import { EditDiff } from "./EditDiff";
import { Text } from "./Text";

// An edit as a diff: lines replaced by others, a file written whole, and
// a long one, cut to its start with the way to the rest.

const meta: Meta = { title: "Components/EditDiff" };
export default meta;

const old = ["func share(total, n int64) int64 {", "\treturn total / n", "}"].join("\n");
const next = ["func share(total, n int64) int64 {", "\t// Half away from zero, as invoice.go rounds.", "\treturn roundDiv(total, n)", "}"].join("\n");
const long = Array.from({ length: 30 }, (_, i) => `line ${i + 1} of a file written whole`).join("\n");

export const Edits: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr)", gap: 20, maxWidth: 560 }}>
      <div>
        <Text as="div" dim>Lines replaced</Text>
        <EditDiff edit={{ path: "api/dashboard.go", old, new: next }} />
      </div>
      <div>
        <Text as="div" dim>A file written whole</Text>
        <EditDiff edit={{ path: "api/round.go", new: next }} />
      </div>
      <div>
        <Text as="div" dim>A long one, cut to 10 lines</Text>
        <EditDiff edit={{ path: "notes.txt", old, new: long }} max={10} />
      </div>
      <div>
        <Text as="div" dim>A line wider than its box scrolls</Text>
        <EditDiff edit={{ path: "api/dashboard.go", new: `// ${"a comment that goes on, ".repeat(8)}` }} />
      </div>
    </div>
  ),
};
