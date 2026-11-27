import type { Meta, StoryObj } from "@storybook/react-vite";
import { AutomationGrants } from "./AutomationGrants";

// What an automation may do through plugins: one let do it without
// asking, one not yet, one its plugin keeps to proposals, and one no
// plugin here proposes.

const meta: Meta<typeof AutomationGrants> = { title: "Automations/AutomationGrants", component: AutomationGrants };
export default meta;

export const EachWay: StoryObj<typeof AutomationGrants> = {
  args: {
    kinds: [
      { kind: "jira:changes", title: "Changes to Jira issues", unattended: true, granted: true },
      { kind: "frame-fixture:things", title: "Fixture things", unattended: true },
      { kind: "jira:issues", title: "Jira issues", unattended: false },
      { kind: "slack:post", missing: true },
    ],
    onGrant: () => {},
  },
};
