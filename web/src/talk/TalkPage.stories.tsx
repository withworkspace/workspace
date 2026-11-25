import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Composer } from "../ui/Composer";
import { Icon } from "../ui/Icon";
import { Status } from "../ui/Status";
import { ToggleButton } from "../ui/ToggleButton";
import { Turn, TurnAvatar, TurnHead, TurnSaid } from "../ui/Turn";
import { TalkAbout } from "./TalkAbout";
import { TalkAnchor } from "./TalkAnchor";
import { TalkBar } from "./TalkBar";
import { TalkLayout } from "./TalkLayout";
import { TalkMain } from "./TalkMain";
import { TalkProposal } from "./TalkProposal";
import { TalkProposalChoice } from "./TalkProposalChoice";
import { TalkProposalWhat } from "./TalkProposalWhat";
import { TalkReply } from "./TalkReply";
import { TalkSettled } from "./TalkSettled";
import { TalkSide } from "./TalkSide";
import { TalkSideItem } from "./TalkSideItem";
import { TalkSideSection } from "./TalkSideSection";
import { TalkSummary } from "./TalkSummary";
import { TalkTurnGroup } from "./TalkTurnGroup";
import { TalkTurns } from "./TalkTurns";
import { talkStatus } from "./words";

// A conversation's page from its parts: its bar (the name, what it's
// about, Details and Resolve), the summary folded over the turns when
// there's no column beside them, the lines or the passage it was asked
// of, the turns with a card under a reply and the cards you settled as
// ws's quiet lines (a run of them close together, one you edited first
// opening under it), and the reply box with the pill that says a reply's
// come below; with the column beside it, and without.

const meta: Meta = { title: "Talk/TalkPage" };
export default meta;

const none = () => {};
const at = "2026-11-25T09:12:00Z";
const head = (by: "you" | "agent" | "ws") => <TurnHead avatar={<TurnAvatar by={by} />} who={{ you: "You", agent: "Agent", ws: "ws" }[by]} at={at} />;

function Page({ side, about = "mission", unseen }: { side?: boolean; about?: "mission" | "pr" | "doc"; unseen?: boolean }) {
  const [text, setText] = useState("");
  return (
    <div style={{ height: "100vh", overflow: "auto" }}>
      <TalkLayout side={side}>
        <TalkBar
          back={{ to: "/conversations", title: "Back to the conversations" }}
          status={<Status s={talkStatus({ status: "open", waiting: true })} iconOnly size={16} />}
          title="Late fees on overdue invoices"
          actions={
            <>
              <ToggleButton variant="ghost" size="small" pressed={!!side} onChange={none}>
                <Icon name="panel" size={12} /> Details
              </ToggleButton>
              <Button>
                <Icon name="check" size={13} />
                Resolve
              </Button>
            </>
          }
        >
          <TalkAbout
            mission={about === "mission" ? { href: "/missions/rate-limit-the-public-api", title: "Rate-limit the public API" } : undefined}
            pr={about === "pr" ? { href: "/reviews/api/41", pr: "api#41", path: "internal/httpapi/idempotency.go", line: 10, endLine: 14 } : undefined}
            doc={about === "doc" ? { href: "/documents", path: "adr/0003-an-invoice-freezes-when-it-is-sent.md", section: "Decision" } : undefined}
            at={at}
          />
        </TalkBar>
        <TalkMain>
          {!side && <TalkSummary text="Late fees would be **a flat fee**, never interest, sent as an invoice of their own." />}
          {about === "pr" && <TalkAnchor excerpt={{ lines: ["func Idempotent(next http.Handler) http.Handler {", "\tvar mu sync.Mutex", "\tseen := map[string]int{}"], from: 10, title: "internal/httpapi/idempotency.go, the lines the question is about" }} />}
          {about === "doc" && <TalkAnchor quote="A sent invoice is never changed: a correction is a credit note and a new invoice." />}
          <TalkTurns>
            <TalkTurnGroup>
              <Turn by="you" head={head("you")}>
                <TurnSaid written>Two studios asked this week for late fees on overdue invoices. Should we do it?</TurnSaid>
              </Turn>
            </TalkTurnGroup>
            <TalkTurnGroup>
              <Turn by="agent" head={head("agent")}>
                <TurnSaid>
                  <Markdown text={"It's worth proposing, and it isn't small.\n\n**What's in the way.** A sent invoice is frozen (ADR 0003)."} />
                </TurnSaid>
              </Turn>
              <TalkProposal state="open" anchor="proposal-1" icon="gavel" label="Decision" onEdit={none} foot={<TalkProposalChoice accept="Record decision" ready onAccept={none} onDecline={none} />}>
                <TalkProposalWhat>Late fees are a flat fee, never interest</TalkProposalWhat>
                <TalkProposalWhat why>Interest changes every day until it's paid, so it can't be invoiced once.</TalkProposalWhat>
              </TalkProposal>
            </TalkTurnGroup>
            <TalkTurnGroup settled>
              <TalkSettled accepted what="mission late-fees" to="/missions/late-fees" said="Accepted: mission late-fees" at={at} />
            </TalkTurnGroup>
            <TalkTurnGroup settled>
              <TalkSettled accepted={false} what="decision: A studio can turn late fees off" edits={"Reworded: **A studio can turn them off**, per client."} at={at} />
            </TalkTurnGroup>
          </TalkTurns>
          <TalkReply onNewReply={unseen ? none : undefined}>
            <Composer value={text} onChange={setText} onSend={none} aria-label="Reply" placeholder="Reply…">
              <Button variant="primary" disabled={!text.trim()}>
                Send
              </Button>
            </Composer>
          </TalkReply>
        </TalkMain>
        {side && (
          <TalkSide>
            <TalkSideSection title="To confirm">
              <ul>
                <li>
                  <TalkSideItem open icon="gavel" name="Late fees are a flat fee, never interest" onClick={none} />
                </li>
              </ul>
            </TalkSideSection>
          </TalkSide>
        )}
      </TalkLayout>
    </div>
  );
}

export const AboutAMission: StoryObj = { render: () => <Page /> };
export const Beside: StoryObj = { render: () => <Page side /> };
export const AboutLines: StoryObj = { render: () => <Page about="pr" unseen /> };
export const AboutAPassage: StoryObj = { render: () => <Page about="doc" /> };
