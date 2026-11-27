import type { Meta, StoryObj } from "@storybook/react-vite";
import { StartedVia } from "./StartedVia";

// What started a conversation and wrote its first message: one of your
// automations (a proposal it made, a question it handed over), or a
// plugin's page.

const meta: Meta<typeof StartedVia> = { title: "Talk/StartedVia", component: StartedVia };
export default meta;

export const ByAnAutomation: StoryObj<typeof StartedVia> = { args: { via: "automation:ready-to-build" } };

export const FromAPlugin: StoryObj<typeof StartedVia> = { args: { via: "jira" } };
