import type { Meta, StoryObj } from "@storybook/react-vite";
import { dismissWhys } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Calls } from "../ui/Calls";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { MoreLink } from "../ui/MoreLink";
import { SendKeys } from "../ui/SendKeys";
import { Spinner } from "../ui/Spinner";
import { Turn, TurnSaid } from "../ui/Turn";
import { PageCalls } from "./PageCalls";
import { PageComposer } from "./PageComposer";
import { PageDismissAs } from "./PageDismissAs";
import { PageFinding } from "./PageFinding";
import { PageFindingActs } from "./PageFindingActs";
import { PageFindingHead } from "./PageFindingHead";
import { PageFindingQuote } from "./PageFindingQuote";
import { PageFindings } from "./PageFindings";
import { PageFindingSuggest } from "./PageFindingSuggest";
import { PageFindingText } from "./PageFindingText";
import { PageFoundOut } from "./PageFoundOut";
import { PagePassHead } from "./PagePassHead";
import { PagePassMeta } from "./PagePassMeta";
import { PagePassPanel } from "./PagePassPanel";
import { PagePassRunning } from "./PagePassRunning";
import { PagePassShown } from "./PagePassShown";
import { PagePassSummary } from "./PagePassSummary";
import { PageTurn } from "./PageTurn";
import { PageTurns } from "./PageTurns";

// A pass over a page, on its tab: never run; reading the page; and what
// it found (one to go to, one whose words have gone from the page, one
// proposing words, one being dismissed, one found out about), with what's
// been settled shown.

const meta: Meta = { title: "Pages/PagePass" };
export default meta;

const noop = () => {};
const how = "Its agent reads the page and what it needs of the workspace. What it says is kept here, beside the page: none of it is written into the page.";

const run = (label: string, primary?: boolean) => (
  <Button variant={primary ? "primary" : undefined}>
    <Icon name={primary ? "play" : "rotate"} size={12} /> {label}
  </Button>
);

export const NeverRun: StoryObj = {
  render: () => (
    <PagePassPanel label="Cold read">
      <PagePassHead about="What an agent would build from this page alone, and everything it had to assume." how="Its agent is given the page and nothing else. What it says is kept here, beside the page: none of it is written into the page." action={run("Run", true)} />
    </PagePassPanel>
  ),
};

export const CantBeRun: StoryObj = {
  render: () => (
    <PagePassPanel label="Guide">
      <PagePassHead
        about="Whether the page follows the workspace's guide to an ADR."
        how={
          <>
            Its agent follows the workspace's skill <span className="mono">adr-guide</span>, and reads what it needs of the workspace. What it says is kept here, beside the page: none of it is written into the page.
          </>
        }
        unavailable="the skill adr-guide isn't in the workspace"
        action={
          <Button variant="primary" disabled>
            <Icon name="play" size={12} /> Run
          </Button>
        }
      />
    </PagePassPanel>
  ),
};

export const Reading: StoryObj = {
  render: () => (
    <PagePassPanel label="Check">
      <PagePassHead
        about="What's open, ambiguous or missing, for someone who'd build from this page."
        how={how}
        action={
          <Button disabled>
            <Spinner size={12} /> Running…
          </Button>
        }
      />
      <PagePassRunning words="Reading the page again">
        <Calls calls={[{ tool: "Read", input: "repos/api/.ground/internal/httpapi/invoices.go", result: "package httpapi" }, { tool: "Grep", input: "RateLimit", pending: true }]} open live />
      </PagePassRunning>
    </PagePassPanel>
  ),
};

export const Found: StoryObj = {
  render: () => (
    <PagePassPanel label="Check">
      <PagePassHead about="What's open, ambiguous or missing, for someone who'd build from this page." how={how} action={run("Run again")} />
      <Banner tone="attention" actions={<Button size="small">Run again</Button>}>
        The page has changed since this ran: what's below is of the page as it was.
      </Banner>
      <PagePassMeta at={<>Ran 4:12 PM</>}>
        <PagePassShown shown onShown={noop} />
      </PagePassMeta>
      <PagePassSummary>
        <Markdown text="Nearly ready to build from: the limits are there, but how they're counted isn't." />
      </PagePassSummary>
      <PageFindings>
        <PageFinding>
          <PageFindingHead>
            <Chip tone="quiet">question</Chip>
            <PageFindingQuote onClick={noop}>a limit per API key</PageFindingQuote>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="Per key, or per studio? A studio with three keys would get three times the limit." />
          </PageFindingText>
          <PageFindingActs>
            <Button size="small">
              <Icon name="check" size={12} /> Done
            </Button>
            <Button variant="ghost" size="small">
              Find out
            </Button>
            <PageDismissAs whys={dismissWhys} onDismiss={noop} />
          </PageFindingActs>
        </PageFinding>
        <PageFinding>
          <PageFindingHead>
            <Chip tone="attention">ambiguity</Chip>
            <PageFindingQuote onClick={noop}>It should be fast, and as it does now for anyone under the limit.</PageFindingQuote>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="“Fast” could be the limit check's own cost, or the API's answer time overall." />
          </PageFindingText>
          <PageFindingSuggest>
            <Markdown text="The limit check adds no more than 1ms to an answer, and nothing changes for requests under the limit." />
          </PageFindingSuggest>
          <PageFindingActs>
            <Button variant="primary" size="small">
              Use these words
            </Button>
            <Button size="small">
              <Icon name="check" size={12} /> Done
            </Button>
            <Button variant="ghost" size="small">
              Find out
            </Button>
            <PageDismissAs whys={dismissWhys} onDismiss={noop} />
          </PageFindingActs>
        </PageFinding>
        <PageFinding>
          <PageFindingHead>
            <Chip tone="attention">ambiguity</Chip>
            <PageFindingQuote lost onClick={noop}>
              said in the answer
            </PageFindingQuote>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="Which headers? The draft IETF ones, or X-RateLimit-*?" />
          </PageFindingText>
          <PageFindingActs>
            <Button size="small">
              <Icon name="check" size={12} /> Done
            </Button>
            <Button variant="ghost" size="small">
              Find out
            </Button>
            <Button variant="ghost" size="small">
              Dismiss
            </Button>
            <span className="dim">Its words aren't on the page any more.</span>
          </PageFindingActs>
        </PageFinding>
        <PageFinding>
          <PageFindingHead>
            <Chip tone="attention">missing</Chip>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="Where the counts are kept: in memory on each instance, or shared?" />
          </PageFindingText>
          <PageFindingActs>
            <Button size="small">
              <Icon name="check" size={12} /> Done
            </Button>
            <PageDismissAs whys={dismissWhys} onDismiss={noop} />
          </PageFindingActs>
          <PageFoundOut>
            <PageTurns flush>
              <PageTurn kind="answer">
                <Turn by="agent" dense landed>
                  <TurnSaid>
                    <Markdown text="Shared: the API runs three instances behind the load balancer, so a count in memory would let a key through three times. Redis is already there for sessions." />
                  </TurnSaid>
                  <PageCalls calls={[{ tool: "Read", input: "repos/api/.ground/cmd/api/main.go", result: "package main" }]} />
                </Turn>
              </PageTurn>
            </PageTurns>
            <PageComposer flush value="" onChange={noop} onSend={noop} tallest={120} placeholder="Ask more, or say what to put in the page…" aria-label="Ask more about this finding" escape="leave">
              <Button size="small">Put it in the page</Button>
              <Button variant="primary" size="small" disabled>
                Ask <SendKeys />
              </Button>
            </PageComposer>
          </PageFoundOut>
        </PageFinding>
      </PageFindings>
      <MoreLink onClick={noop} aria-expanded>
        Hide 2 settled
      </MoreLink>
      <PageFindings settled>
        <PageFinding look="settled">
          <PageFindingHead>
            <Chip tone="quiet">question</Chip>
            <PageFindingQuote>the dashboard is exempt</PageFindingQuote>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="Is the dashboard a key like any other?" />
          </PageFindingText>
          <PageFindingActs>
            <span className="dim">Done</span>
            <Button variant="ghost" size="small">
              Open it again
            </Button>
          </PageFindingActs>
        </PageFinding>
        <PageFinding look="settled">
          <PageFindingHead>
            <Chip tone="attention">ambiguity</Chip>
          </PageFindingHead>
          <PageFindingText>
            <Markdown text="Which headers?" />
          </PageFindingText>
          <PageFindingActs>
            <span className="dim">Dismissed: it's wrong</span>
            <Button variant="ghost" size="small">
              Open it again
            </Button>
          </PageFindingActs>
        </PageFinding>
      </PageFindings>
    </PagePassPanel>
  ),
};
