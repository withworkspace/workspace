import type { Meta, StoryObj } from "@storybook/react-vite";
import { ErrorCard } from "./ErrorCard";

// An error card: what didn't load, as the server said it (made a
// sentence), with Try again where there's a way to, and a title of its
// own.

const meta: Meta = { title: "Components/ErrorCard" };
export default meta;

export const Every: StoryObj = {
  render: () => (
    <div className="page narrow">
      <ErrorCard error="error: mission not found" />
      <ErrorCard error="the daemon didn't answer" retry={() => {}} />
      <ErrorCard title="Couldn't open this file" error="open web/src/ui/Callout.tsx: no such file or directory" retry={() => {}} />
      <ErrorCard error="" />
    </div>
  ),
};
