import type { Meta, StoryObj } from "@storybook/react-vite";
import { Markdown } from "../markdown/LazyMarkdown";
import { DocumentPeek } from "./DocumentPeek";
import { docPeekOf } from "./peek";

// A document as it's read beside another page (z, on a link to it):
// where it is, how long it is, and what it says, at the width of the
// dock. On its repo's default branch; as it is in a capsule; and one a
// mission records, with a path too long for its line.

const meta: Meta = { title: "Documents/DocumentPeek" };
export default meta;

const pane: React.CSSProperties = { width: 388, padding: 16, border: "1px solid var(--line)", borderRadius: 10, background: "var(--raised)" };

const adr = [
  "# Queue the webhooks",
  "",
  "A webhook that fails is tried again from a queue, with backoff, and not in the request that made it.",
  "",
  "## Why",
  "",
  "A customer's endpoint that's down held our request open until it timed out: one slow endpoint could use every worker.",
  "",
  "- Tries are at 1, 5, 30 minutes, then hourly for a day.",
  "- After a day it's given up, and the customer's told.",
  "",
  "```go",
  "func (q *Queue) Retry(ctx context.Context, d Delivery) error",
  "```",
  "",
  "| Try | After |",
  "| --- | --- |",
  "| 1 | 1 min |",
  "| 2 | 5 min |",
].join("\n");

const of = docPeekOf({ text: adr, in: { repo: "records", worktree: ".ground", path: "adr/0007-queue-the-webhooks.md" } } as Parameters<typeof docPeekOf>[0], "records:adr/0007-queue-the-webhooks.md", () => "Records");

export const OnTheDefaultBranch: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <DocumentPeek repo={of.repo} path={of.path} words={of.words}>
          <Markdown text={of.text} />
        </DocumentPeek>
      </div>
    </div>
  ),
};

export const InACapsule: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <DocumentPeek repo="Records" capsule="webhook-retries" path={of.path} words={of.words}>
          <Markdown text={of.text} />
        </DocumentPeek>
      </div>
    </div>
  ),
};

export const OfAMission: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <DocumentPeek repo="API" capsule="webhook-retries" path="docs/internal/operations/webhooks/retrying-failed-deliveries-with-backoff.md" words={1} mission={{ title: "Retry failed webhooks with backoff", to: "/missions/retry-failed-webhooks-with-backoff" }}>
          <Markdown text="Soon." />
        </DocumentPeek>
      </div>
    </div>
  ),
};
