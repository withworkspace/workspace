import type { Meta, StoryObj } from "@storybook/react-vite";
import { PullRequestMerge } from "./PullRequestMerge";

// Merging a pull request of yours from ws: one that's ready, a stack whole,
// one only waiting (left to merge by itself once it can), one that's been
// left to, and one being asked.

const meta: Meta<typeof PullRequestMerge> = { title: "Reviews/PullRequestMerge", component: PullRequestMerge, args: { onMerge: () => {}, onStop: () => {} } };
export default meta;

type Story = StoryObj<typeof PullRequestMerge>;

export const Ready: Story = { args: { offer: "now", primary: true } };
export const Stack: Story = { args: { offer: "now", stack: true, primary: true } };
export const Waiting: Story = { args: { offer: "ready" } };
export const MergesWhenReady: Story = { args: { offer: "on" } };
export const Merging: Story = { args: { offer: "now", primary: true, busy: true } };
