import type { Meta, StoryObj } from "@storybook/react-vite";
import { Loading } from "./Loading";

// Loading: shown at once (delay 0), saying what's loading, and as a page
// has it, which is nothing for a moment and then there.

const meta: Meta = { title: "Components/Loading" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 8 }}>
      <Loading delay={0} />
      <Loading delay={0} what="Reading the files…" />
      <Loading what="After a moment: Diffing their sources…" />
    </div>
  ),
};
