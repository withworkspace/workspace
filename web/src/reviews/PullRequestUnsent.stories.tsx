import type { Meta, StoryObj } from "@storybook/react-vite";
import { PullRequestUnsent } from "./PullRequestUnsent";

// What GitHub didn't take of what was said from a pull request's page: a
// reply with what it said, and a thread it wouldn't resolve; and one being
// sent again.

const meta: Meta<typeof PullRequestUnsent> = { title: "Reviews/PullRequestUnsent", component: PullRequestUnsent };
export default meta;

const unsent = [
  { id: 4, doing: "your reply on cache.go:3", body: "Fixed in the next commit: the key now carries the tenant.", error: "replying on GitHub: the thread is locked", at: "2026-11-10T12:00:00Z" },
  { id: 5, doing: "resolving the thread on tenant.go:12", error: "resolving on GitHub: Resource not accessible by personal access token", at: "2026-11-10T12:01:00Z" },
];

export const Refused: StoryObj<typeof PullRequestUnsent> = { args: { unsent, onResend: () => {}, onDiscard: () => {} } };

export const SendingAgain: StoryObj<typeof PullRequestUnsent> = { args: { unsent, busy: 4, onResend: () => {}, onDiscard: () => {} } };
