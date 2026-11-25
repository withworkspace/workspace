import type { Meta, StoryObj } from "@storybook/react-vite";
import { TrackedDoc } from "./LazyTracked";

// A document with suggested edits in place: a change in a line, the one
// shown, and a block taken out; then one being written in.

const meta: Meta = { title: "Editor/TrackedDoc" };
export default meta;

const none = () => {};
const frame = (children: React.ReactNode) => <div style={{ padding: 32, maxWidth: 760 }}>{children}</div>;

const tracked = `# Limit each key

Each key may make a hundred requests a minute.

Keys over it are told to wait.

The end.
`;

export const Suggested: StoryObj = {
  render: () =>
    frame(
      <TrackedDoc
        text={tracked}
        changes={[
          { id: "c1", by: "agent", from: "a hundred requests", to: "sixty requests" },
          { id: "c2", by: "you", from: "Keys over it are told to wait.", to: "" },
        ]}
        focus="c1"
        onSettle={none}
      />,
    ),
};

export const Settling: StoryObj = {
  render: () => frame(<TrackedDoc text={tracked} changes={[{ id: "c1", by: "c2", from: "a hundred requests", to: "sixty requests" }]} settling="c1" onSettle={none} />),
};
