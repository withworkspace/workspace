import type { Meta, StoryObj } from "@storybook/react-vite";
import { Markdown } from "../markdown/LazyMarkdown";
import { PullRequestChecks } from "./PullRequestChecks";
import { PullRequestPeek } from "./PullRequestPeek";
import { PullRequestPeekLayer } from "./PullRequestPeekLayer";
import { PullRequestSize } from "./PullRequestSize";

// A pull request as it's read beside another page (z, on a link to it):
// how it stands with you, whose it is and where it's going, how big it
// is, its checks, the stack it's in and what it says it does, at the
// width of the dock. First its parts alone, then whole: one that needs
// your review, a layer of a stack, and a draft of yours with nothing
// said of it.

const meta: Meta = { title: "Reviews/PullRequestPeek" };
export default meta;

const pane: React.CSSProperties = { width: 388, padding: 16, border: "1px solid var(--line)", borderRadius: 10, background: "var(--raised)" };

const needs = { icon: "circle-dot", tone: "attention", words: "Needs your review" } as const;
const waiting = { icon: "clock", tone: "waiting", words: "Waiting on them" } as const;
const approved = { icon: "check-circle", tone: "done", words: "Approved" } as const;
const draft = { icon: "circle-dashed", tone: "quiet", words: "Draft" } as const;

const body = ["Each key has so many requests a minute; over it, the gateway answers `429` with `Retry-After`.", "", "- The limit is the key's plan's, read once a minute.", "- Counting is in Redis, a window a key.", "", "```go", "func (l *Limiter) Allow(key string) (bool, time.Duration)", "```"].join("\n");

export const Parts: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14, width: 388 }}>
      <div style={{ display: "flex", gap: 18, alignItems: "center" }}>
        <PullRequestSize additions={212} deletions={38} files={9} />
        <PullRequestSize additions={1} deletions={0} files={1} />
        <PullRequestSize additions={12840} deletions={9310} />
      </div>
      <div style={{ display: "flex", gap: 18, alignItems: "center", fontSize: "var(--text-sm)" }}>
        <PullRequestChecks checks="success" />
        <PullRequestChecks checks="failure" />
        <PullRequestChecks checks="pending" />
      </div>
      <ol style={{ display: "grid", gap: 2, margin: 0, padding: 0, listStyle: "none" }}>
        <PullRequestPeekLayer number={44} title="Count requests a key, in Redis" status={approved} to="/reviews/api/44" />
        <PullRequestPeekLayer number={45} title="Answer 429 over the limit, with when to try again" status={needs} to="/reviews/api/45" here />
        <PullRequestPeekLayer number={46} title="Say so in the docs" status={waiting} to="/reviews/api/46" />
      </ol>
    </div>
  ),
};

export const NeedsYourReview: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <PullRequestPeek status={needs} author="mira" branch="rate-limit" base="main" additions={212} deletions={38} files={9} checks="success" updated={new Date(Date.now() - 42 * 60_000).toISOString()} mission={{ title: "Rate-limit the public API", to: "/missions/rate-limit-the-public-api" }}>
          <Markdown text={body} />
        </PullRequestPeek>
      </div>
    </div>
  ),
};

export const ALayerOfAStack: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <PullRequestPeek
          status={needs}
          author="mira"
          branch="rate-limit/answer-429-with-when-to-try-again"
          base="rate-limit/count-requests-a-key"
          additions={96}
          deletions={12}
          files={4}
          checks="failure"
          updated={new Date(Date.now() - 3 * 3600_000).toISOString()}
          layers={[
            { number: 44, title: "Count requests a key, in Redis", status: approved, to: "/reviews/api/44" },
            { number: 45, title: "Answer 429 over the limit, with when to try again", status: needs, to: "/reviews/api/45", here: true },
            { number: 46, title: "Say so in the docs", status: waiting, to: "/reviews/api/46" },
          ]}
        >
          <Markdown text={body} />
        </PullRequestPeek>
      </div>
    </div>
  ),
};

export const ADraftOfYours: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <PullRequestPeek status={draft} author="you" mine draft branch="spike-token-bucket" base="main" additions={540} deletions={0} files={3} checks="pending" />
      </div>
    </div>
  ),
};
