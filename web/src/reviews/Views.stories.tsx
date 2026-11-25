import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { GuideText as GuideTextData, Said } from "../api";
import { ReviewFindingActions } from "../review/ReviewFindingActions";
import { ReviewViewBody } from "../review/ReviewViewBody";
import { Button } from "../ui/Button";
import { Excerpt } from "../ui/Excerpt";
import { GuideBox } from "./GuideBox";
import { GuideFoot } from "./GuideFoot";
import { GuideMark } from "./GuideMark";
import { GuideText } from "./GuideText";
import { GuideWriting } from "./GuideWriting";
import { OverviewMission } from "./OverviewMission";
import { PassCard } from "./PassCard";
import { PassFinding } from "./PassFinding";
import { PassFindingLink } from "./PassFindingLink";
import { PassFindings } from "./PassFindings";
import { PassList } from "./PassList";
import { PassMark } from "./PassMark";
import { PassNote } from "./PassNote";
import { PassSettled } from "./PassSettled";
import { PassShow } from "./PassShow";
import { PassStanding } from "./PassStanding";
import { PassSummary } from "./PassSummary";
import { SaidHead } from "./SaidHead";
import { SaidItem } from "./SaidItem";
import { SaidList } from "./SaidList";
import { SaidReply } from "./SaidReply";

// A review's views beside its changes, from their parts: its passes (one
// no go with what it found, one running, one that can't run), its guide
// (written, being written, couldn't be), the mission a capsule's for, and
// what's been said on GitHub with a box to say more.

const meta: Meta = { title: "Reviews/Views" };
export default meta;

const none = () => {};
const at = "2026-11-25T09:00:00Z";

function Passes() {
  const [dismissing, setDismissing] = useState(false);
  const [show, setShow] = useState(true);
  return (
    <ReviewViewBody label="Passes" hidden={false} read>
      <h2>Passes</h2>
      <p style={{ display: "flex", gap: 6 }}>
        <PassMark standing={{ status: "nogo", open: 2, blocking: 1 }} running={false} title="Bugs: no go" />
        <PassMark standing={{ status: "go", open: 0, blocking: 0 }} running={false} title="Tests: go" />
        <PassMark standing={{ status: "none", open: 0, blocking: 0 }} running title="Docs: running" />
      </p>
      <PassList>
        <PassCard
          standing={<PassStanding standing={{ status: "nogo", open: 2, blocking: 1 }}>No go · 1 blocking</PassStanding>}
          title="Bugs"
          about="What could go wrong at runtime"
          tools={
            <>
              <PassShow dim checked={show} onChange={setShow} title="Draw what it found on the lines it's about">
                In the diff
              </PassShow>
              <Button size="small">Run again</Button>
            </>
          }
        >
          <PassNote kind="moved"> It's been pushed to since this ran (at 0a96360): what it found may not stand. Run it again.</PassNote>
          <PassSummary text="Two places the limit can be passed: a `0` key, and a retry inside the window." at={at} />
          <PassFindings>
            <PassFinding
              blocks
              kind={<span className="kind k-found k-blocking">bug</span>}
              where={<PassFindingLink onClick={none}>limit/limit.go:12–14</PassFindingLink>}
              said={<span className="dim">· suggests a change</span>}
              end={<ReviewFindingActions dismissing={dismissing} canAsk onTake={none} onDismissing={setDismissing} onDismiss={() => setDismissing(false)} onAsk={none} />}
              lines={<Excerpt lines={["if key == \"\" {", "  return nil", "}"]} from={12} />}
              text="An empty key is never counted, so it's never limited."
            />
            <PassFinding kind={<span className="kind k-found k-addressed">Dealt with</span>} where={<span className="dim">The change as a whole</span>} end={<Button size="small">Resolve the comment</Button>} text="The retry header is in seconds now." />
          </PassFindings>
          <PassSettled summary="1 settled: 1 taken, 0 dismissed">
            <PassFindings>
              <PassFinding settled kind={<span className="kind k-found">nit</span>} where={<span className="dim mono">limit/limit.go:3</span>} said={<span className="dim">Taken into your review</span>} end={<Button variant="ghost" size="small">Open again</Button>} text="Name the constant." />
            </PassFindings>
          </PassSettled>
        </PassCard>
        <PassCard standing={<PassStanding standing={{ status: "go", open: 0, blocking: 0 }} spin>Running again</PassStanding>} title="Tests" about="Whether it's tested" tools={<Button size="small" disabled>Running…</Button>}>
          <PassNote kind="steps">3 steps: Read limit/limit_test.go</PassNote>
        </PassCard>
        <PassCard standing={<PassStanding standing={{ status: "none", open: 0, blocking: 0 }}>Not run</PassStanding>} title="Docs" about="Whether the docs say so" tools={<Button size="small" unavailable="it needs an agent">Run</Button>}>
          <PassNote>It can't be run: it needs an agent.</PassNote>
        </PassCard>
      </PassList>
    </ReviewViewBody>
  );
}

export const ThePasses: StoryObj = { render: () => <Passes /> };

const guide: GuideTextData = {
  does: "Counts each key's requests in Redis, and answers 429 once there are too many.",
  differs: ["It says it limits by IP; it limits by key."],
  groups: [
    { name: "The limit", why: "Read this first: the rest calls it.", files: [{ path: "limit/limit.go", note: "where the count is kept" }] },
    { name: "Lockfiles", skim: true, files: [{ path: "go.sum" }] },
  ],
};

function Guide() {
  const [guided, setGuided] = useState(true);
  return (
    <ReviewViewBody label="Guide" hidden={false} read>
      <h2>
        Guide <GuideMark guide={{ id: "g", failed: "no model" } as never} /> <GuideMark guide={{ id: "g", guide, moved: true } as never} />
      </h2>
      <GuideBox>
        <PassNote kind="moved"> It's been pushed to since this was written (at 0a96360): files it doesn't place come first.</PassNote>
        <GuideText g={guide} />
        <GuideFoot guided={guided} at={at} running={false} onGuided={setGuided} onRun={none} />
      </GuideBox>
      <h2>Being written</h2>
      <GuideBox>
        <GuideWriting since={at} thinking given={{ files: ["a", "b", "go.sum"], left: [{ path: "go.sum", why: "lockfile" }] } as never} again={false} />
        <GuideText g={{ does: guide.does, groups: guide.groups.slice(0, 1) }} writing />
      </GuideBox>
    </ReviewViewBody>
  );
}

export const TheGuide: StoryObj = { render: () => <Guide /> };

const said: Said[] = [
  { id: "1", by: "tomas", kind: "approved", body: "Reads well. One for later: the map only ever grows.", at, url: "https://github.com" },
  { id: "2", by: "jun", kind: "changes", body: "The retry header is a date here.", at, stale: true },
  { id: "3", by: "ci-bot", bot: true, kind: "comment", body: "Coverage: 81%", at },
  { id: "4", by: "", mine: true, kind: "comment", body: "Thanks: fixed.", at, sending: true },
];

function Discussion() {
  const [text, setText] = useState("");
  return (
    <ReviewViewBody label="Discussion" hidden={false} read>
      <h2>Mission</h2>
      <OverviewMission slug="rate" title="Rate-limit the public API" repo="api" name="rate" branch="ws/rate" />
      <h2>On GitHub</h2>
      <SaidList>
        {said.map((it) => (
          <SaidItem key={it.id} head={<SaidHead it={it} />} body={it.body} bot={it.bot} />
        ))}
      </SaidList>
      <SaidReply text={text} busy={false} onText={setText} onSend={none} />
      <SaidReply text="Couldn't send this" busy={false} error="GitHub said no" onText={none} onSend={none} />
    </ReviewViewBody>
  );
}

export const TheDiscussion: StoryObj = { render: () => <Discussion /> };
