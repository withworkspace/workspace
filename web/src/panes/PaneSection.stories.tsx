import type { Meta, StoryObj } from "@storybook/react-vite";
import { PaneSection } from "./PaneSection";
import { Text } from "../ui/Text";

// One part of what a pane says of its thing, under a quiet label: with
// how many it holds, as a number or as words, and without.

const meta: Meta = { title: "Chrome/PaneSection" };
export default meta;

export const Sections: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, width: 388 }}>
      <PaneSection title="Plan" count="2 of 5">
        <Text as="div" dim>Its steps.</Text>
      </PaneSection>
      <PaneSection title="Crew" count={2}>
        <Text as="div" dim>Who's on it.</Text>
      </PaneSection>
      <PaneSection title="What it says it does">
        <Text as="div" dim>Its description.</Text>
      </PaneSection>
    </div>
  ),
};
