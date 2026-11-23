import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { PageBar, pageBarTabsClass } from "./PageBar";
import { Status } from "./Status";
import { Tab, Tabs } from "./Tabs";
import { ToggleButton } from "./ToggleButton";

// Page bars: over a page with views of it (their tabs on the bar's rule,
// its name renamed in place), over one with none (what's done to it at
// the bar's end), with a name too long for the row, and while the page
// loads.

const meta: Meta = { title: "Components/PageBar" };
export default meta;

const doing = { icon: "pr", tone: "progress", words: "Ready to merge" } as const;
const yours = { icon: "reply", tone: "attention", words: "Your turn" } as const;

function WithViews() {
  const [tab, setTab] = useState("summary");
  const [title, setTitle] = useState("Rate-limit the public API");
  return (
    <PageBar
      back={{ to: "/", title: "Missions (esc)", label: "Back to the missions" }}
      status={<Status s={doing} iconOnly size={16} />}
      title={title}
      onRename={setTitle}
      renameLabel="Rename the mission"
      tabs={
        <Tabs className={pageBarTabsClass()} value={tab} onChange={setTab} aria-label="Mission">
          <Tab value="summary">Summary</Tab>
          <Tab value="brief">Brief</Tab>
          <Tab value="work">Work</Tab>
          <Tab value="verify">
            Verify <Count tone="attention">1</Count>
          </Tab>
          <Tab value="log">Log</Tab>
        </Tabs>
      }
    >
      <Button variant="ghost" size="small">
        <Icon name="pin" size={12} /> Pin
      </Button>
    </PageBar>
  );
}

function Stub({ title }: { title?: string }) {
  const [side, setSide] = useState(true);
  return (
    <PageBar
      back={{ to: "/conversations", title: "Back to the conversations" }}
      status={title ? <Status s={yours} iconOnly size={16} /> : undefined}
      title={title}
      actions={
        title && (
          <>
            <ToggleButton variant="ghost" size="small" pressed={side} onChange={setSide}>
              <Icon name="panel" size={12} /> Details
            </ToggleButton>
            <Button>
              <Icon name="check" size={13} /> Resolve
            </Button>
          </>
        )
      }
    >
      {title && (
        <span className="dim" style={{ flex: "none", whiteSpace: "nowrap" }}>
          2h ago
        </span>
      )}
    </PageBar>
  );
}

const col = { display: "grid", gap: 28, gridTemplateColumns: "minmax(0, 1fr)" } as const;

export const Bars: StoryObj = {
  render: () => (
    <div style={col}>
      <WithViews />
      <Stub title="Where do we round money?" />
      <Stub title="Two studios asked this week for late fees on overdue invoices. Should we do it, and what would it take to charge them without changing an invoice that's been sent?" />
      <Stub />
    </div>
  ),
};
