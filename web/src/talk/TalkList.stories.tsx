import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import { TalkNew } from "./TalkNew";
import { TalkRow } from "./TalkRow";
import { talkStatus } from "./words";

// The conversations list from its parts: where one's started (and the
// same come to from "New mission", flashing), then a row for each in
// every way one stands: your turn, proposals to confirm, about a pull
// request or a document, become a mission, answering, resolved, and the
// keys' cursor on one.

const meta: Meta = { title: "Talk/TalkList" };
export default meta;

function New({ fresh }: { fresh?: boolean }) {
  const [text, setText] = useState("");
  return (
    <TalkNew
      fresh={fresh}
      value={text}
      onChange={setText}
      onSend={() => {}}
      rows={2}
      tallest={280}
      aria-label="Start a conversation"
      placeholder={fresh ? "Describe the mission you have in mind: what, and why. The agent reads the code and proposes a brief." : "What are you thinking about? An idea, a problem, a question about the code…"}
      hint={
        <>
          <Icon name="eye" size={12} />
          The agent reads the repos, the missions and GitHub, and changes nothing. It can propose a mission, a decision or a document.
        </>
      }
    >
      <Button variant="primary" disabled={!text.trim()}>
        Start
      </Button>
    </TalkNew>
  );
}

const at = "2026-11-25T09:12:00Z";

export const List: StoryObj = {
  render: () => (
    <div className="page narrow">
      <New />
      <Rows role="listbox" aria-label="Conversations">
        <TalkRow href="/conversations/w1" status={talkStatus({ status: "open", waiting: true })} title="Late fees on overdue invoices" yourTurn proposed={1} summary="Worth proposing: a fee would go out as an invoice of its own, since a sent one is frozen (ADR 0003)." at={at} />
        <TalkRow href="/conversations/w2" selected status={talkStatus({ status: "open" })} title="Where do we round money, and is it the same everywhere?" summary="Once, half away from zero, per line; totals add rounded lines." at={at} />
        <TalkRow href="/conversations/w3" status={talkStatus({ status: "open", answering: true })} title="Why is the key stored before the handler runs?" pr="api#41" at={at} />
        <TalkRow href="/conversations/w4" status={talkStatus({ status: "open" })} title="Does this stop late fees?" doc={{ name: "0003-an-invoice-freezes-when-it-is-sent.md", title: "The document it's about: records:adr/0003-an-invoice-freezes-when-it-is-sent.md, § Decision" }} at={at} />
        <TalkRow href="/conversations/w5" status={talkStatus({ status: "resolved" })} title="Export as CSV, or as a spreadsheet too?" became="export-invoices-as-csv" summary="CSV first; a spreadsheet is a follow-up if anyone asks." at={at} />
      </Rows>
    </div>
  ),
};

export const Fresh: StoryObj = {
  render: () => (
    <div className="page narrow">
      <New fresh />
    </div>
  ),
};
