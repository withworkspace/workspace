import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { dismissWhys } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Calls } from "../ui/Calls";
import type { Call } from "../ui/CallLine";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { SendKeys } from "../ui/SendKeys";
import { Spinner } from "../ui/Spinner";
import { Turn, TurnSaid } from "../ui/Turn";
import { PageComposer } from "./PageComposer";
import { PageDismissAs } from "./PageDismissAs";
import { PageFinding } from "./PageFinding";
import { PageFindingActs } from "./PageFindingActs";
import { PageFindingHead } from "./PageFindingHead";
import { PageFindingQuote } from "./PageFindingQuote";
import { PageFindingText } from "./PageFindingText";
import { PageNoteFrom } from "./PageNoteFrom";
import { PageNotes } from "./PageNotes";
import { PagePanel } from "./PagePanel";
import { PagePanelFoot } from "./PagePanelFoot";
import { PagePanelHead } from "./PagePanelHead";
import { PageSourceChip } from "./PageSourceChip";
import { PageSources } from "./PageSources";
import { PageStale } from "./PageStale";
import { PageTurn } from "./PageTurn";
import { PageTurns } from "./PageTurns";
import { PageUnstarted } from "./PageUnstarted";
import { PageVersion } from "./PageVersion";
import { Text } from "../ui/Text";
import { RichText } from "../ui/RichText";

// What's opened over a page's words: a cell's panel (written, with its
// agent's notes and the files it read; asked to change, a second time;
// out of date; one its agent couldn't fill; a question, answered; its
// agent at work), the box that asks for a change, and a pass's finding
// opened from its words.

const meta: Meta = { title: "Pages/PagePanel" };
export default meta;

const reads: Call[] = [
  { tool: "Read", input: "repos/api/.ground/cmd/api/main.go", result: "package main" },
  { tool: "Read", input: "repos/api/.ground/internal/httpapi/invoices.go", result: "package httpapi" },
];

function Over({ children }: { children: ReactNode }) {
  return <div style={{ position: "relative", height: 560, maxWidth: 520 }}>{children}</div>;
}

const at = { top: 0, left: 0 };
const noop = () => {};

function Composer({ question, written = true }: { question?: boolean; written?: boolean }) {
  return (
    <PageComposer value="" onChange={noop} onSend={noop} tallest={160} placeholder={question ? "Ask more, or say what to put in the page…" : written ? "Say what to change: group them by service, drop the last column… Or ask about it." : "Add to what you asked for…"} aria-label="Change this cell, or ask about it" escape="leave">
      {question ? (
        <>
          <Button size="small">Put it in the page</Button>
          <Button variant="primary" size="small" disabled>
            Ask <SendKeys />
          </Button>
        </>
      ) : (
        <>
          <Button size="small" disabled>
            Just ask
          </Button>
          <Button variant="primary" size="small" disabled>
            Change <SendKeys />
          </Button>
        </>
      )}
    </PageComposer>
  );
}

function Foot({ id, question }: { id: string; question?: boolean }) {
  return (
    <PagePanelFoot id={id}>
      {!question && <Button variant="link">Keep as text</Button>}
      <Button variant="link">Remove</Button>
    </PagePanelFoot>
  );
}

function Sources({ moved }: { moved?: boolean }) {
  return (
    <PageSources>
      <PageSourceChip to="/files/api?path=cmd/api/main.go" title="api/cmd/api/main.go. Open it in Files.">
        <Icon name="file" size={11} />
        api/cmd/api/main.go
      </PageSourceChip>
      <PageSourceChip to={moved ? undefined : "/files/records?path=proposals/usage-based-billing.md"} moved={moved} title="records/proposals/usage-based-billing.md">
        <Icon name={moved ? "alert" : "file"} size={11} />
        records/…/usage-based-billing.md
      </PageSourceChip>
    </PageSources>
  );
}

export const Written: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="a table of the routes and who calls each" onClose={noop} />
        <PageTurns>
          <PageTurn kind="version">
            <PageVersion>
              <Icon name="sparkle" size={12} />
              <span>Written</span>
              <Text dim>4:11 PM</Text>
            </PageVersion>
            <Calls calls={reads} />
            <PageNotes>
              <Markdown text="`POST /webhooks/test` isn't documented anywhere: I took it from the router. Worth checking it's meant to be public." />
            </PageNotes>
          </PageTurn>
        </PageTurns>
        <Sources />
        <Composer />
        <Foot id="w4" />
      </PagePanel>
    </Over>
  ),
};

export const WrittenAgain: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="a short list of what changes for integrators" onClose={noop} />
        <PageTurns>
          <PageTurn kind="version">
            <PageVersion>
              <Icon name="pencil" size={11} />
              <span>Your words, as they were</span>
              <Button variant="link">Use this</Button>
            </PageVersion>
          </PageTurn>
          <PageTurn kind="version">
            <PageVersion>
              <Icon name="sparkle" size={12} />
              <span>Version 1</span>
              <Text dim>4:11 PM</Text>
              <Button variant="link">Use this</Button>
            </PageVersion>
            <PageNotes old>
              <Markdown text="I left out the dashboard: it's exempt for now." />
            </PageNotes>
          </PageTurn>
          <PageTurn>
            <Turn by="you" dense landed>
              <TurnSaid>
          <RichText text="Shorter: two lines." />
        </TurnSaid>
            </Turn>
            <PageVersion yours>
              <Icon name="pencil" size={11} />
              <span>With your edits to it</span>
              <Button variant="link">Use this</Button>
            </PageVersion>
          </PageTurn>
          <PageTurn kind="version">
            <PageVersion>
              <Icon name="sparkle" size={12} />
              <span>Version 2</span>
              <Text dim>4:12 PM</Text>
            </PageVersion>
          </PageTurn>
        </PageTurns>
        <Composer />
        <Foot id="w8" />
      </PagePanel>
    </Over>
  ),
};

export const OutOfDate: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="what each plan's limit should be, from the pricing notes" onClose={noop} />
        <PageTurns>
          <PageTurn kind="version">
            <PageVersion>
              <Icon name="sparkle" size={12} />
              <span>Written</span>
              <Text dim>4:11 PM</Text>
            </PageVersion>
          </PageTurn>
        </PageTurns>
        <Sources moved />
        <PageStale words="usage-based-billing.md has changed since this was written.">
          <Button size="small">Write it again</Button>
        </PageStale>
        <Composer />
        <Foot id="w5" />
      </PagePanel>
    </Over>
  ),
};

export const Unfilled: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="the dashboard's retry code" onClose={noop} />
        <PageTurns>
          <PageTurn kind="failed">
            <Icon name="alert" size={12} /> The agent couldn't answer: the run stopped at a step that failed.
          </PageTurn>
        </PageTurns>
        <PageUnstarted words="It couldn't be filled.">
          <Button size="small">Try again</Button>
        </PageUnstarted>
        <Composer written={false} />
        <Foot id="w6" />
      </PagePanel>
    </Over>
  ),
};

export const Answered: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="do webhooks count against the limit?" onClose={noop} />
        <PageTurns>
          <PageTurn kind="answer">
            <Turn by="agent" dense landed>
              <TurnSaid>
                <Markdown text="No: webhooks are what Sorrel sends, not requests made of the API. A retry of a delivery doesn't count either." />
              </TurnSaid>
              <Calls calls={reads.slice(0, 1)} />
            </Turn>
          </PageTurn>
        </PageTurns>
        <Composer question />
        <Foot id="w7" question />
      </PagePanel>
    </Over>
  ),
};

export const Filling: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="This cell">
        <PagePanelHead prompt="a sequence diagram of a throttled request" onClose={noop} />
        <PageTurns>
          <PageTurn kind="filling">
            <PageVersion>
              <Spinner size={12} />
              <span>Filling it in</span>
            </PageVersion>
            <Calls calls={[...reads.slice(0, 1), { tool: "Grep", input: "RateLimit", pending: true }]} open live />
          </PageTurn>
        </PageTurns>
        <Composer written={false} />
        <Foot id="w9" />
      </PagePanel>
    </Over>
  ),
};

export const AskingForAChange: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} label="Ask for these words to be changed">
        <PageComposer value="" onChange={noop} onSend={noop} tallest={160} placeholder="What should change here? Tighten it, make it a table, add what's missing…" aria-label="Ask for these words to be changed">
          <Button variant="primary" size="small" disabled>
            Ask <SendKeys />
          </Button>
        </PageComposer>
        <PagePanelFoot hint="They become a cell: an agent writes them again, and your words are kept to put back." />
      </PagePanel>
    </Over>
  ),
};

export const AFindingFromItsWords: StoryObj = {
  render: () => (
    <Over>
      <PagePanel at={at} note label="From Check">
        <PageNoteFrom pass="Check" onPass={noop} onClose={noop} />
        <PageFinding look="in">
          <PageFindingHead>
            <Chip tone="quiet">question</Chip>
            <PageFindingQuote>a limit per API key</PageFindingQuote>
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
      </PagePanel>
    </Over>
  ),
};
