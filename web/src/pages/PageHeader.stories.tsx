import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Spinner } from "../ui/Spinner";
import { Tab } from "../ui/Tabs";
import { ToggleButton } from "../ui/ToggleButton";
import { PageFrame } from "./PageFrame";
import { PageHeader } from "./PageHeader";
import { PageRetry } from "./PageRetry";
import { PageSaveState, type Saving } from "./PageSaveState";
import { PageSelectPop } from "./PageSelectPop";
import { PageTabFailed } from "./PageTabFailed";
import { PageTabs } from "./PageTabs";

// The top of a page: back to Documents, its name and kind, planning (or
// not), what's done with it, and where saving stands, in each of its
// states; then the page's tab and its passes' (one running, one with
// findings open, one that failed); and what's under words selected.

const meta: Meta = { title: "Pages/PageHeader" };
export default meta;

const noop = () => {};

function Head({ saving, planning }: { saving: Saving; planning?: "on" | "failed" }) {
  return (
    <PageHeader back={{ to: "/documents", label: "Documents" }} id="p2">
      <Chip tone="quiet">ADR</Chip>
      <span className="spacer" />
      {planning === "on" && (
        <Chip tone="progress">
          <Spinner size={11} /> Planning: reading api/cmd/api/main.go
        </Chip>
      )}
      {planning === "failed" && <PageRetry onClick={noop}>Couldn't plan it: try again</PageRetry>}
      <ToggleButton variant="ghost" size="small" pressed={false} onChange={noop}>
        <Icon name="plus" size={12} /> Add
      </ToggleButton>
      <Button variant="ghost" size="small">
        <Icon name="pencil" size={12} /> Draft it
      </Button>
      <Button variant="ghost" size="small">
        <Icon name="sparkle" size={12} /> Change <Keys k="mod+⏎" />
      </Button>
      <ToggleButton variant="ghost" size="small" pressed onChange={noop}>
        <Icon name="panel" size={12} /> Map
      </ToggleButton>
      <Button variant="ghost" size="small">
        <Icon name="file" size={12} /> File as…
      </Button>
      <PageSaveState state={saving} retry={noop} />
      <Button variant="ghost" size="small">
        Remove
      </Button>
    </PageHeader>
  );
}

function Tabs() {
  const [tab, setTab] = useState("page");
  return (
    <PageTabs value={tab} onChange={setTab}>
      <Tab value="page">Page</Tab>
      <Tab value="check">
        Check
        <Count tone="attention">3</Count>
      </Tab>
      <Tab value="formal">
        Formalise
        <Spinner size={11} />
      </Tab>
      <Tab value="cold">
        Cold read
        <PageTabFailed />
      </Tab>
    </PageTabs>
  );
}

export const Saved: StoryObj = {
  render: () => (
    <PageFrame>
      <Head saving="saved" />
      <Tabs />
    </PageFrame>
  ),
};

export const EachSavingState: StoryObj = {
  render: () => (
    <PageFrame>
      <Head saving="saving" planning="on" />
      <Head saving="changed" />
      <Head saving="failed" planning="failed" />
    </PageFrame>
  ),
};

export const WordsSelected: StoryObj = {
  render: () => (
    <PageFrame>
      <div style={{ position: "relative", height: 120 }}>
        <p>
          Integrators hammer <span style={{ background: "var(--accent-soft)" }}>GET /invoices when they sync</span>, and a single studio's sync slowed the API for everyone.
        </p>
        <PageSelectPop top={40} left={120}>
          <Button variant="ghost" size="small">
            <Icon name="question" size={12} /> Ask
          </Button>
          <Button variant="ghost" size="small">
            <Icon name="sparkle" size={12} /> Change <Keys k="mod+⏎" />
          </Button>
        </PageSelectPop>
      </div>
    </PageFrame>
  ),
};
