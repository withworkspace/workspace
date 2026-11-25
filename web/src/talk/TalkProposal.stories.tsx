import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { buttonClass } from "../ui/Button";
import { Change } from "../ui/Change";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";
import { TalkPluginItem } from "./TalkPluginItem";
import { TalkPluginItems } from "./TalkPluginItems";
import { TalkProposal } from "./TalkProposal";
import { TalkProposalChanges, talkProposalChangedClass } from "./TalkProposalChanges";
import { TalkProposalChoice } from "./TalkProposalChoice";
import { TalkProposalEdit } from "./TalkProposalEdit";
import { TalkProposalFrame } from "./TalkProposalFrame";
import { TalkProposalFrames } from "./TalkProposalFrames";
import { TalkProposalFrom } from "./TalkProposalFrom";
import { TalkProposalMeta } from "./TalkProposalMeta";
import { TalkProposalPlan } from "./TalkProposalPlan";
import { TalkProposalSettled } from "./TalkProposalSettled";
import { TalkProposalStale } from "./TalkProposalStale";
import { TalkProposalWarnings } from "./TalkProposalWarnings";
import { TalkProposalWhat } from "./TalkProposalWhat";
import { TalkProposalWhere } from "./TalkProposalWhere";
import { TalkProposalWhole } from "./TalkProposalWhole";

// What a conversation's agent proposes, a card under its reply, from its
// parts. Open: a decision, as it's confirmed, being edited, being done,
// and revising one before it; a mission with its brief whole; a brief that
// starts its mission, with its plan folded; a review comment; crew. Then
// what came of them: done (with a way to what it made), declined,
// superseded, withdrawn. A plugin's: items ticked to accept, one alone,
// and once tried (made, not done, not accepted). An artifact: running,
// beside the version it revises, with newer versions since and what
// won't load.

const meta: Meta = { title: "Talk/TalkProposal" };
export default meta;

const none = () => {};
const brief = "## Goal\nA proposal in records for late fees on overdue invoices.\n\n## Acceptance\n- It says how a fee is charged without changing a sent invoice.";
const plan = "1. Work: **Write the proposal.**\n2. Review: **Read it.**";
const choice = (accept: string, more: Partial<Parameters<typeof TalkProposalChoice>[0]> = {}) => <TalkProposalChoice accept={accept} ready onAccept={none} onDecline={none} {...more} />;
const Cards = ({ children }: { children: ReactNode }) => <div style={{ display: "grid", gap: 16, maxWidth: 720, padding: 16 }}>{children}</div>;
const decision = (
  <>
    <TalkProposalWhat>Late fees are a flat fee, never interest</TalkProposalWhat>
    <TalkProposalWhat why>Interest changes every day until it's paid, so it can't be invoiced once.</TalkProposalWhat>
  </>
);

export const Open: StoryObj = {
  render: () => (
    <Cards>
      <TalkProposal state="open" icon="gavel" label="Decision" onEdit={none} foot={choice("Record decision")}>
        {decision}
      </TalkProposal>
      <TalkProposal state="open" icon="gavel" label="Decision" foot={choice("Record decision")}>
        <TalkProposalEdit>
          <TextArea value="Late fees are a flat fee, never interest" rows={2} onChange={none} aria-label="The decision" />
          <TextArea value="" rows={2} onChange={none} placeholder="Why" aria-label="Why" />
        </TalkProposalEdit>
      </TalkProposal>
      <TalkProposal state="open" icon="gavel" label="Decision" foot={choice("Record decision", { busy: "Recording…" })}>
        {decision}
      </TalkProposal>
      <TalkProposal
        state="open"
        icon="gavel"
        label="Decision"
        revises={2}
        onEdit={none}
        changes={
          <TalkProposalChanges n={2}>
            <Change className={talkProposalChangedClass} wrap struck hunks={[{ was: ["Late fees are interest by the day"], now: ["Late fees are a flat fee, never interest"] }]} />
          </TalkProposalChanges>
        }
        foot={choice("Record decision", { replying: true })}
      >
        {decision}
      </TalkProposal>
      <TalkProposal state="open" icon="flag" label="Mission" title="Propose late fees on overdue invoices" onEdit={none} foot={choice("Create mission")}>
        <TalkProposalWhole text={brief} />
        <TalkProposalMeta items={["About api, records", "Lifts records/late-fees"]} />
      </TalkProposal>
      <TalkProposal state="open" icon="pencil" label="Mission brief" title="Archive invoices older than seven years" onEdit={none} foot={choice("Confirm and start")}>
        <TalkProposalWhole text={brief} />
        <TalkProposalPlan text={plan} />
        <TalkProposalMeta items={["Lifts api/archive", "Renamed archive-old-invoices"]} />
      </TalkProposal>
      <TalkProposal state="open" icon="message" label="Review comment" onEdit={none} foot={choice("Add to my review")}>
        <TalkProposalWhere kind="blocking" pr="api#41" href="/reviews/api/41" path="internal/httpapi/idempotency.go" lines="12–14" />
        <TalkProposalWhat said>A key that's reused with a different body should be refused, not answered from the cache.</TalkProposalWhat>
      </TalkProposal>
      <TalkProposal state="open" icon="play" label="Crew" onEdit={none} foot={choice("Start crew")}>
        <TalkProposalWhat said>{"Move the counts into Redis.\n\nKeep the in-memory store for tests."}</TalkProposalWhat>
        <TalkProposalMeta items={["In api/rate-limit"]} />
      </TalkProposal>
    </Cards>
  ),
};

const open = (to: string) => (
  <Link to={to} className={buttonClass()}>
    Open it <Icon name="arrow-right" size={13} />
  </Link>
);

export const Settled: StoryObj = {
  render: () => (
    <Cards>
      <TalkProposal
        state="accepted"
        icon="flag"
        label="Mission"
        title="Propose late fees on overdue invoices"
        foot={
          <>
            <TalkProposalSettled done icon="check-circle" size={14}> Mission created</TalkProposalSettled>
            {open("/missions/late-fees")}
          </>
        }
      >
        <TalkProposalWhole text={brief} />
      </TalkProposal>
      <TalkProposal state="declined" icon="gavel" label="Decision" foot={<TalkProposalSettled icon="x"> Declined</TalkProposalSettled>}>
        {decision}
      </TalkProposal>
      <TalkProposal
        state="superseded"
        icon="gavel"
        label="Decision"
        foot={
          <TalkProposalSettled icon="arrow-down">
            <span>
              Superseded by <a href="#proposal-3">proposal 3</a>, proposed after it: nothing to confirm here
            </span>
          </TalkProposalSettled>
        }
      >
        {decision}
      </TalkProposal>
      <TalkProposal state="withdrawn" icon="play" label="Crew" foot={<TalkProposalSettled icon="x"> Withdrawn: the plan changed</TalkProposalSettled>}>
        <TalkProposalWhat said>Move the counts into Redis.</TalkProposalWhat>
      </TalkProposal>
    </Cards>
  ),
};

const from = <TalkProposalFrom plugin="frame-fixture" proposes="things" />;

export const Plugin: StoryObj = {
  render: () => (
    <Cards>
      <TalkProposal state="open" icon="box" label="Fixture things" title="4 things for the shelf" foot={choice("Accept 3 of 4")}>
        <TalkPluginItems>
          <TalkPluginItem n={1} title="Teapot" on="THING-1" pick={{ picked: true, onPick: none }} rows={[{ label: "Note", value: "On the shelf, beside the kettle." }]} />
          <TalkPluginItem n={2} title="Tea cosy" pick={{ picked: true, onPick: none }} rows={[{ label: "Beside", value: "item 1, Teapot" }]} />
          <TalkPluginItem n={3} title="Anvil" pick={{ picked: false, onPick: none }} markdown={"Too heavy for **the shelf**."} />
          <TalkPluginItem n={4} title="The note on THING-2, rewritten" pick={{ picked: true, onPick: none }} rows={[{ label: "Note", was: "Dusty.", value: "Dusted, and its wick trimmed." }]} />
        </TalkPluginItems>
        {from}
      </TalkProposal>
      <TalkProposal state="open" icon="box" label="Fixture things" title="Kettle descaler" foot={choice("Accept")}>
        <TalkPluginItems>
          <TalkPluginItem n={1} alone rows={[{ label: "Note", value: "Under the sink." }]} markdown={"A long note.\n\n".repeat(40)} long />
        </TalkPluginItems>
        {from}
      </TalkProposal>
      <TalkProposal state="open" icon="box" label="Fixture things" title="4 things for the shelf" foot={choice("Try the 1 left", { decline: "Decline the rest", declineTitle: "What's done stands: the rest is declined" })}>
        <TalkPluginItems>
          <TalkPluginItem n={1} title="Teapot" made="THING-4" say="Put on the shelf." />
          <TalkPluginItem n={2} title="Tea cosy" made />
          <TalkPluginItem n={3} title="Anvil" pick={{ picked: true, onPick: none }} problem="the shelf won't take an anvil" />
          <TalkPluginItem n={4} title="A lamp" left />
        </TalkPluginItems>
        {from}
      </TalkProposal>
    </Cards>
  ),
};

const page = "data:text/html,<body style='font:14px system-ui'><h1>Late fees</h1><p>Flat fee: £15 after 14 days</p></body>";

export const Artifact: StoryObj = {
  render: () => (
    <Cards>
      <TalkProposal state="open" icon="artifact" label="Artifact" title="Late fee settings" onEdit={none} foot={choice("Add artifact")}>
        <TalkProposalWhat said>A studio's late fee: on or off, how much, and after how many days.</TalkProposalWhat>
        <TalkProposalFrames>
          <TalkProposalFrame src={page} title="Late fee settings, as proposed" />
        </TalkProposalFrames>
        <TalkProposalMeta items={["Adds .ws/artifacts/late-fee-settings, on this machine", "1 file, 1 KB"]} />
        <TalkProposalWarnings warnings={["https://fonts.example.com/a.css: the frame loads nothing from outside"]} />
      </TalkProposal>
      <TalkProposal state="open" icon="artifact" label="A version of a1" title="Where Export goes" onEdit={none} foot={choice("Add as v4")}>
        <TalkProposalWhat said>Export beside the count, with the fee settings under it.</TalkProposalWhat>
        <TalkProposalStale>Proposed on v2; v3 has been added since. Adding it still bases it on v2.</TalkProposalStale>
        <TalkProposalFrames two>
          <TalkProposalFrame src={page} caption="v2, which it revises" title="Where Export goes, v2" />
          <TalkProposalFrame src={page} caption="Proposed" title="Where Export goes, as proposed" />
        </TalkProposalFrames>
      </TalkProposal>
      <TalkProposal state="open" icon="artifact" label="Artifact" foot={choice("Add artifact", { ready: false })}>
        <TalkProposalEdit>
          <Input value="" onChange={none} placeholder="Its title" aria-label="Its title" />
          <TextArea value="" rows={2} onChange={none} placeholder="What it tries out" aria-label="Its note" />
          <div className="dim">The HTML is the agent's: ask it for another version to change that.</div>
        </TalkProposalEdit>
      </TalkProposal>
    </Cards>
  ),
};
