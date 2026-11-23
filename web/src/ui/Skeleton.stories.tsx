import type { Meta, StoryObj } from "@storybook/react-vite";
import { Skeleton } from "./Skeleton";

// Skeletons: the four rows it draws unasked, a few, and a page's worth.

const meta: Meta = { title: "Components/Skeleton" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page narrow" style={{ display: "grid", gap: 24 }}>
      <Skeleton />
      <Skeleton rows={2} />
      <Skeleton rows={8} />
    </div>
  ),
};
