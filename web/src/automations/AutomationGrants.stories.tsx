import type { Meta, StoryObj } from "@storybook/react-vite";
import { AutomationGrants } from "./AutomationGrants";

// What an automation may do through plugins: one let do it without
// asking, one not yet, one its plugin cautions against (granted anyway,
// and not), and one no plugin here proposes.

const meta: Meta<typeof AutomationGrants> = { title: "Automations/AutomationGrants", component: AutomationGrants };
export default meta;

export const EachWay: StoryObj<typeof AutomationGrants> = {
  args: {
    kinds: [
      { kind: "jira:changes", title: "Changes to Jira issues", unattended: true, granted: true },
      { kind: "frame-fixture:things", title: "Fixture things", unattended: true },
      { kind: "jira:changes", title: "Jira changes", caution: "A move or a new assignee may set off the board's own rules, which nobody sees happen." },
      { kind: "jira:issues", title: "Jira issues", granted: true },
      { kind: "slack:post", missing: true },
    ],
    onGrant: () => {},
  },
};
