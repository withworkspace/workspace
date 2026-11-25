import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Link } from "react-router";
import type { Posting } from "../api";
import { ReviewLayout } from "../review/ReviewLayout";
import { ReviewVerdicts } from "../review/ReviewVerdicts";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Status, type StatusInfo } from "../ui/Status";
import { TextArea } from "../ui/TextArea";
import { PullRequestLayers } from "./PullRequestLayers";
import { PullRequestMeta, pullRequestMetaChipClass } from "./PullRequestMeta";
import { PullRequestMetaChecks } from "./PullRequestMetaChecks";
import { PullRequestName } from "./PullRequestName";
import { PullRequestPosting } from "./PullRequestPosting";
import { PullRequestReviewer } from "./PullRequestReviewer";
import { PullRequestSubmit } from "./PullRequestSubmit";
import { pullRequestLeftAloneClass, PullRequestSubmitLayer } from "./PullRequestSubmitLayer";

// A pull request's review from its parts: its name (a layer, the whole
// stack), where it stands, the stack's layers down the side, and the
// dialog that posts it to GitHub, with what each layer would post.

const meta: Meta = { title: "Reviews/PullRequest" };
export default meta;

const none = () => {};
const needs: StatusInfo = { icon: "circle-dot", tone: "attention", words: "Needs your review" };
const approved: StatusInfo = { icon: "check-circle", tone: "done", words: "Approved" };
const posting: Posting = {
  repo: "api",
  number: 45,
  title: "Answer 429",
  url: "https://github.com/sorrel/api/pull/45",
  commit: "88cf2940aa",
  event: "REQUEST_CHANGES",
  body: "Close: the retry header needs another look.",
  comments: [{ id: "c1", path: "limit/limit.go", side: "RIGHT", startLine: 12, line: 14, body: "Retry-After is in seconds, not a date." }],
  loose: 1,
};

function Head() {
  const [whole, setWhole] = useState(false);
  return (
    <div style={{ height: 520 }}>
      <ReviewLayout
        side={
          <PullRequestLayers
            layers={[
              { number: 44, title: "Count requests a key, in Redis", words: "approved", status: approved, to: "/reviews/api/44" },
              { number: 45, title: "Answer 429 over the limit, with when to try again", words: "needs your review", status: needs, to: "/reviews/api/45", here: !whole },
              { number: 46, title: "Say so in the docs", words: "needs your review", status: needs, to: "/reviews/api/46", going: true },
            ]}
            merged={1}
            title="Stack 3 on GitHub"
            about="3 layers"
            whole={whole}
            onWhole={() => setWhole((w) => !w)}
          />
        }
        onResize={none}
        onResetWidth={none}
      >
        <div style={{ padding: "16px 0", display: "grid", gap: 16 }}>
          <h1>{whole ? <PullRequestName title="Rate-limit the public API" whole={{ layers: 3 }} /> : <PullRequestName title="Answer 429 over the limit" at="api#45" layer={{ n: 2, of: 3 }} />}</h1>
          <PullRequestMeta>
            <Status s={needs} />
            <span>
              mira · <span className="mono">rate-429</span> <Icon name="arrow-right" size={11} /> <span className="mono">rate-redis</span>
            </span>
            <PullRequestMetaChecks checks="failure" />
            <PullRequestReviewer login="tomas" kind="approved" />
            <PullRequestReviewer login="jun" kind="changes" stale />
            <PullRequestReviewer login="ana" kind="dismissed" />
            <Link to="/missions/rate" className={pullRequestMetaChipClass("quiet")}>
              <Icon name="flag" size={11} /> Rate-limit the public API
            </Link>
            <Link to="/conversations/c1" className={pullRequestMetaChipClass("attention")}>
              <Icon name="message" size={11} /> Answered: what calls this?
            </Link>
          </PullRequestMeta>
        </div>
      </ReviewLayout>
    </div>
  );
}

export const NameAndStack: StoryObj = { render: () => <Head /> };

function Submit() {
  return (
    <PullRequestSubmit
      title="Submit the review of the stack"
      onClose={none}
      intro="Submitting posts to GitHub, as you. It's the one thing here that leaves your machine, and a review can't be unsent, only dismissed: what's below is exactly what goes."
      all={
        <>
          <Button>
            <Icon name="check-circle" size={13} /> Approve every layer <Keys k="A" />
          </Button>
          <span className="dim">
            <Keys k="[" /> <Keys k="]" /> pick a layer
          </span>
        </>
      }
      start={<Button variant="ghost">Discard the draft</Button>}
      end={
        <>
          <Button variant="ghost">Cancel</Button>
          <Button variant="primary">
            Post 2 reviews to GitHub <Keys k="mod+⏎" />
          </Button>
        </>
      }
    >
      <PullRequestSubmitLayer
        n={1}
        title="Count requests a key, in Redis"
        at="api#44"
        said="no comments"
        state={
          <span className="settled-line done">
            <Icon name="check-circle" size={13} /> Posted
          </span>
        }
        focus={false}
        posted
        onFocus={none}
      />
      <PullRequestSubmitLayer n={2} title="Answer 429 over the limit" at="api#45" said="1 comment (1 blocking)" focus posted={false} onFocus={none}>
        <ReviewVerdicts label="Verdict for #45" value="changes" keyed onPick={none} />
        <TextArea value="Close: the retry header needs another look." rows={2} onChange={none} aria-label="Summary" />
        <PullRequestPosting p={posting} />
      </PullRequestSubmitLayer>
      <PullRequestSubmitLayer n={3} title="Say so in the docs" at="api#46" said="no comments" focus={false} posted={false} onFocus={none}>
        <ReviewVerdicts label="Verdict for #46" keyed={false} onPick={none} after={<span className={`dim ${pullRequestLeftAloneClass}`}>Left alone</span>} />
      </PullRequestSubmitLayer>
    </PullRequestSubmit>
  );
}

export const Submitting: StoryObj = { render: () => <Submit /> };

export const PostingNothing: StoryObj = { render: () => <PullRequestPosting p={{ ...posting, event: "APPROVE", body: "", comments: [], loose: 0 }} /> };
