import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Empty } from "../../ui/Empty";
import { Inline } from "../../ui/Inline";
import { Keys } from "../../ui/Keys";
import { Printed } from "../../ui/Printed";
import { MissionVerify } from "./MissionVerify";
import { MissionVerifyApprove } from "./MissionVerifyApprove";
import { MissionVerifyAsks } from "./MissionVerifyAsks";
import { MissionVerifyBundle } from "./MissionVerifyBundle";
import { MissionVerifyBundles } from "./MissionVerifyBundles";
import { MissionVerifyCheck } from "./MissionVerifyCheck";
import { MissionVerifyClear } from "./MissionVerifyClear";
import { MissionVerifyCommand } from "./MissionVerifyCommand";
import { MissionVerifyEvidence } from "./MissionVerifyEvidence";
import { MissionVerifyFold } from "./MissionVerifyFold";
import { MissionVerifyGroup } from "./MissionVerifyGroup";
import { MissionVerifyHead } from "./MissionVerifyHead";
import { MissionVerifyJudged } from "./MissionVerifyJudged";
import { MissionVerifyKept } from "./MissionVerifyKept";
import { MissionVerifyLines } from "./MissionVerifyLines";
import { MissionVerifyList } from "./MissionVerifyList";
import { MissionVerifyMenu, type MissionVerifyMenuGroup } from "./MissionVerifyMenu";
import { MissionVerifyNote } from "./MissionVerifyNote";
import { MissionVerifyPanes } from "./MissionVerifyPanes";
import { MissionVerifyRow } from "./MissionVerifyRow";
import { MissionVerifySaid } from "./MissionVerifySaid";
import { MissionVerifyShot } from "./MissionVerifyShot";
import { MissionVerifyStanding } from "./MissionVerifyStanding";
import { MissionVerifySummary } from "./MissionVerifySummary";
import { MissionVerifyTally } from "./MissionVerifyTally";
import { MissionVerifyYours } from "./MissionVerifyYours";
import { MissionVerifyYourself } from "./MissionVerifyYourself";

// A mission's Verify tab, from its parts: a bundle that's yours to
// approve, with a check open and what's yours of it; one you sent back,
// which another has taken the place of; the pane once you're all clear;
// the tab with nothing left; and a check's row in each state.

const meta: Meta = { title: "Mission/MissionVerify" };
export default meta;

const none = () => {};
const shot = (said: string, ink: string) =>
  `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="160"><rect width="480" height="160" fill="#f4f1ea"/><text x="24" y="60" font-family="monospace" font-size="20">$ ./demo marlow</text><text x="24" y="100" font-family="monospace" font-size="20" fill="${ink}">${said}</text></svg>`)}`;

const others: MissionVerifyMenuGroup[] = [
  { key: "back", title: "Sent back", bundles: [{ id: "b1", title: "Step 2: Limit each key", standing: <MissionVerifyStanding tone="attention">Sent back, 2 checks</MissionVerifyStanding> }] },
  { key: "approved", title: "Approved", bundles: [{ id: "b3", title: "Step 1: Design it", standing: <MissionVerifyStanding tone="done">Approved</MissionVerifyStanding> }] },
  { key: "stale", title: "Stale", back: true, bundles: [{ id: "b4", title: "Of api/spike", standing: <MissionVerifyStanding>Stale</MissionVerifyStanding> }] },
];

function Row() {
  return (
    <MissionVerifyBundles label="To approve" attention>
      <MissionVerifyBundle id="b2" title="Step 2: Limit each key" standing={<MissionVerifyStanding tone="attention">Yours to approve</MissionVerifyStanding>} left={2} selected onPick={none} />
      <MissionVerifyBundle id="b5" title="Step 4: Say so in the docs" standing={<MissionVerifyStanding tone="attention">Yours to approve</MissionVerifyStanding>} left={1} onPick={none} />
      <MissionVerifyMenu groups={others} label="3 other bundles" onPick={none} end />
    </MissionVerifyBundles>
  );
}

const summary =
  "Checks 1 and 2 are met. Over the limit the gateway answers 429 with `Retry-After`, in seconds, and a key under it is let through.\n\nHow the docs name the header is yours to say: check 3 shows the page as it reads now.\n\nThe limit is counted in Redis with a window that slides, so a burst at the turn of a minute isn't let through twice. That was measured with 240 requests sent across the turn of a minute, of which 120 were answered 200 and 120 were answered 429.";

function Yours() {
  const [note, setNote] = useState<string | undefined>("The docs say `X-RateLimit-Reset`: it's `Retry-After` that's sent.");
  const [looked, setLooked] = useState(false);
  return <MissionVerifyYours crew="c3" left looked={looked} onLook={() => setLooked(!looked)} note={note} onNote={setNote} />;
}

function Check({ open }: { open?: boolean }) {
  return (
    <MissionVerifyCheck
      verdict={<Chip tone="danger">Unmet</Chip>}
      meta="Check 2, c3, its verifier"
      title="Over the limit, a client is told when to try again"
      said={<Inline text="The gateway answers 429 with `Retry-After`. The docs name another header." />}
      points={[<Inline text="`curl -i` over the limit: `429`, `Retry-After: 41`." />, <Inline text="The docs page: `X-RateLimit-Reset`." />]}
      yours={open && <Yours />}
    >
      <MissionVerifyJudged tone="danger">
        <strong>The judge says unmet</strong>: the page names a header the gateway doesn't send.
      </MissionVerifyJudged>
      <Printed>{"$ curl -i https://api.sorrel.test/v1/invoices\nHTTP/2 429\nretry-after: 41\ncontent-type: application/json"}</Printed>
      <MissionVerifyKept to="/runs" />
      <MissionVerifyEvidence pair caption="A name, greeted" madeBy="pnpm shots -- limits">
        <MissionVerifyShot kind="image" url={shot("hello", "#9a5800")} what="A name, greeted" label="Before, on ground" />
        <MissionVerifyShot kind="image" url={shot("hello, marlow", "#1b7a43")} what="A name, greeted" label="After, in the capsule" />
      </MissionVerifyEvidence>
      <MissionVerifyEvidence caption="What the tests printed">
        <MissionVerifyShot kind="text" url="" what="What the tests printed">
          <Printed>{"$ go test ./...\nok  \tgateway/limit\t0.212s"}</Printed>
        </MissionVerifyShot>
      </MissionVerifyEvidence>
      <MissionVerifyLines lines={["gateway/limit/limit.go:41-88", "docs/limits.md:12"]} />
      <MissionVerifyYourself
        items={[
          <>
            <MissionVerifyCommand text="make dev" /> then ask for more than 120 a minute
          </>,
          <span>
            <span className="dim">Look for</span> the 429, and its header
          </span>,
        ]}
      />
      <MissionVerifyAsks crew="c3" asks={["Name `Retry-After` in the docs, as the gateway sends it.", "Say the limit a key has, on the same page."]} onAsk={open ? none : undefined} />
    </MissionVerifyCheck>
  );
}

// A bundle that's yours to approve: the row, its head, its checks with
// two left for you, and the check that's open, with what you say isn't
// right of it.
export const Tab: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionVerify>
        <Row />
        <MissionVerifyPanes
          side={
            <>
              <MissionVerifyHead
                id="b2"
                title="Step 2: Limit each key"
                current
                meta={
                  <>
                    From c3, a verifier who didn't do the work, of <span className="mono">api/rate-limit</span> at 3f9c2ab, 4 minutes ago
                  </>
                }
              >
                <MissionVerifySummary text={summary} />
                <MissionVerifyTally
                  replaces={
                    <>
                      It took the place of <Button variant="link">b1</Button>.
                    </>
                  }
                >
                  <Chip tone="done">2 met</Chip>
                  <Chip tone="danger">1 unmet</Chip>
                  <Chip tone="attention">1 yours</Chip>
                </MissionVerifyTally>
                <MissionVerifyFold title="Not verified" items={["Safari: it isn't installed here.", "A key with more than one limit."]} />
                <MissionVerifyFold title="Noticed" items={["The first request after a restart is slow."]} />
                <MissionVerifyFold title="Try it" items={[<MissionVerifyCommand text="make dev" />, "Look for the 429 over 120 requests a minute"]} />
              </MissionVerifyHead>
              <MissionVerifyList>
                <MissionVerifyGroup title="Left for you" n={2} attention>
                  <MissionVerifyRow icon="x-circle" tone="danger" title="Over the limit, a client is told when to try again" line="Not right, you say" kind="wanted" selected onChoose={none} />
                  <MissionVerifyRow icon="user" tone="attention" title="The docs name the header a client reads" line="Left for you by c3 · 1 picture" kind="wanted" onChoose={none} />
                </MissionVerifyGroup>
                <MissionVerifyApprove says="1 check goes back to c3, with what the bundle showed and what you said. The one they hand over next takes its place." onSettle={none}>
                  Send back to c3 (1) <Keys k="mod+⏎" />
                </MissionVerifyApprove>
                <MissionVerifyGroup title="You approved" n={1} fold={{ open: true, onToggle: none }}>
                  <MissionVerifyRow icon="check-circle" tone="done" title="A key under the limit is let through" kind="approved" onChoose={none} />
                </MissionVerifyGroup>
                <MissionVerifyGroup title="Settled without you" n={1}>
                  <MissionVerifyRow icon="check-circle" tone="done" title={<Inline text="`go test ./...` passes" />} line="ws ran it" kind="settled" onChoose={none} />
                </MissionVerifyGroup>
              </MissionVerifyList>
            </>
          }
        >
          <Check open />
        </MissionVerifyPanes>
      </MissionVerify>
    </div>
  ),
};

// One you sent back, which another has taken the place of: what you said,
// its checks as they stood, and nothing of it left to say.
export const SentBack: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionVerify>
        <MissionVerifyBundles label="Nothing to approve">
          <MissionVerifyBundle id="b1" title="Step 2: Limit each key" standing={<MissionVerifyStanding>b2 took its place</MissionVerifyStanding>} selected other onPick={none} />
          <MissionVerifyMenu groups={others} label="3 bundles" onPick={none} end />
        </MissionVerifyBundles>
        <MissionVerifyPanes
          side={
            <>
              <MissionVerifyHead id="b1" title="Step 2: Limit each key" meta="From c2, who did the work it shows, of api/rate-limit at a81b07c, yesterday">
                <MissionVerifyNote tone="quiet" icon="info" actions={<Button>Open b2</Button>}>
                  b2 has taken its place.
                </MissionVerifyNote>
                <MissionVerifyNote tone="attention" icon="reply" title="You sent it back yesterday">
                  <MissionVerifySaid notes={[{ check: 2, text: "The docs say `X-RateLimit-Reset`: it's `Retry-After` that's sent." }, { text: "And say the limit a key has." }]} />
                </MissionVerifyNote>
                <MissionVerifyNote tone="done" icon="check-circle">
                  You approved it 2 hours ago.
                </MissionVerifyNote>
                <MissionVerifyNote tone="attention" title="Stale">
                  Its capsule has moved on to <span className="mono">3f9c2ab</span> since. What this shows is of <span className="mono">a81b07c</span>.
                </MissionVerifyNote>
                <MissionVerifySummary text="Check 2 is unmet, so this goes back." />
                <MissionVerifyTally>
                  <Chip tone="done">1 met</Chip>
                  <Chip tone="danger">1 unmet</Chip>
                </MissionVerifyTally>
              </MissionVerifyHead>
              <MissionVerifyList>
                <MissionVerifyGroup title="Left for you" n={1} attention>
                  <MissionVerifyRow icon="x-circle" tone="danger" title="Over the limit, a client is told when to try again" line="c2 says unmet · 2 pictures" selected onChoose={none} />
                </MissionVerifyGroup>
                <MissionVerifyGroup title="You approved" n={2} fold={{ open: false, onToggle: none }} />
                <MissionVerifyGroup title="Settled without you" n={1}>
                  <MissionVerifyRow icon="check-circle" tone="done" title={<Inline text="`go test ./...` passes" />} line="ws ran it" kind="settled" onChoose={none} />
                </MissionVerifyGroup>
              </MissionVerifyList>
            </>
          }
        >
          <Check />
        </MissionVerifyPanes>
      </MissionVerify>
    </div>
  ),
};

// The check's pane once the last check left for you is approved.
export const AllClear: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionVerify>
        <MissionVerifyPanes
          side={
            <MissionVerifyList>
              <MissionVerifyApprove says="Approving b2 passes step 2." onSettle={none}>
                Approve b2 <Keys k="mod+⏎" />
              </MissionVerifyApprove>
              <MissionVerifyApprove plain says="Approving b2 passes step 2. 2 checks are still left for you." onSettle={none}>
                Approve b2 <Keys k="mod+⏎" />
              </MissionVerifyApprove>
            </MissionVerifyList>
          }
        >
          <MissionVerifyClear id="b2" after="Approving b2 passes step 2." onApprove={none} onAgain={none}>
            You've approved the 2 checks left for you. 2 more were met without you.
          </MissionVerifyClear>
        </MissionVerifyPanes>
      </MissionVerify>
    </div>
  ),
};

// The whole tab when nothing's yours to approve and no bundle's open.
export const NothingLeft: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionVerify done>
        <Empty big done icon="check" title="Nothing left to approve" action={<MissionVerifyMenu groups={others} label="Look at the 3 bundles" onPick={none} />}>
          You approved 1 and sent 1 back; 1 is stale. A bundle that needs you will show here, and on the tab.
        </Empty>
      </MissionVerify>
    </div>
  ),
};

// A check's row: left for you in a bundle that's yours to approve (and
// open), as it stands in one that isn't, approved by you, settled
// without you; and where a bundle can stand.
export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, width: 400 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 10 }}>
        <MissionVerifyStanding tone="attention">Yours to approve</MissionVerifyStanding>
        <MissionVerifyStanding tone="attention">Sent back, 2 checks</MissionVerifyStanding>
        <MissionVerifyStanding tone="done">Approved</MissionVerifyStanding>
        <MissionVerifyStanding>b2 took its place</MissionVerifyStanding>
      </div>
      <div>
        <MissionVerifyRow icon="user" tone="attention" title="The docs name the header a client reads, and it's the one the gateway sends" line="Left for you by c3 · 1 picture" kind="wanted" onChoose={none} />
        <MissionVerifyRow icon="question" tone="info" title="A second tab sees the change" line="c3 can't tell" kind="wanted" selected onChoose={none} />
        <MissionVerifyRow icon="x-circle" tone="danger" title="Over the limit, a client is told when to try again" line="c2 says unmet · 2 pictures" onChoose={none} />
        <MissionVerifyRow icon="check-circle" tone="done" title="A key under the limit is let through" line="c3 says met" kind="approved" onChoose={none} />
        <MissionVerifyRow icon="check-circle" tone="done" title={<Inline text="`go test ./...` passes" />} line="ws ran it" kind="settled" onChoose={none} />
        <MissionVerifyRow icon="check-circle" tone="done" title={<Inline text="`go vet ./...` passes" />} line="ws ran it" kind="settled" selected onChoose={none} />
      </div>
    </div>
  ),
};
