import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { Button } from "../../ui/Button";
import { Copyable } from "../../ui/Copyable";
import { Icon } from "../../ui/Icon";
import { Status } from "../../ui/Status";
import { Time } from "../../ui/Time";
import { MissionCapsuleBadges } from "../MissionCapsuleBadges";
import { MissionData } from "../MissionData";
import { MissionSideSection } from "../MissionSideSection";
import { MissionSide } from "./MissionSide";
import { MissionSideCameFrom } from "./MissionSideCameFrom";
import { MissionSideCapsule } from "./MissionSideCapsule";
import { MissionSideFact, MissionSideFacts } from "./MissionSideFacts";
import { MissionSideMore } from "./MissionSideMore";
import { MissionSideRun } from "./MissionSideRun";
import { Side } from "./Side";

// A mission's details: as missions ws gave (mission/fixtures) have them,
// side by side, and each part's states drawn from props.

const meta: Meta = { title: "Mission/MissionSide" };
export default meta;

const none = () => {};

// The details' column, as wide as the page gives it.
const column = (children: ReactNode, key?: string) => (
  <div key={key} style={{ width: 256, flex: "none", padding: "16px 8px 0" }}>
    {children}
  </div>
);

function Taken({ name }: { name: string }) {
  const m = takenMissions.find((x) => x.name === name);
  if (!m) return <p className="dim">No mission {name} among the fixtures.</p>;
  return (
    <MissionData.Provider value={m.d}>
      <Side d={m.d} selected={(k) => k === `c:${m.d.capsules[0]?.repo}/${m.d.capsules[0]?.name}`} openArtifact={none} review={none} toWork={none} reload={async () => {}} updated={m.d.events.at(-1)?.time} />
    </MissionData.Provider>
  );
}

// Missions in different courts: crew at work and an orchestrator's run, a
// pull request to merge, crew asking, parked, being briefed, ended. Each
// has its first capsule selected.
export const Missions: StoryObj = {
  render: () => {
    storyWorkspace();
    return (
      <RefsProvider>
        <div style={{ display: "flex", gap: 24, alignItems: "start", padding: 16, background: "var(--bg)" }}>
          {["reminders-for-overdue-invoices", "rate-limit-the-public-api", "work-across-repos", "invoices-in-more-than-one-currency", "archive-invoices-older-than-seven-years.briefed", "which-index-is-missing"].map((n) => column(<Taken name={n} />, n))}
        </div>
      </RefsProvider>
    );
  },
};

// Over another tab, open: a card at the right of what it's over.
export const Over: StoryObj = {
  render: () => (
    <div style={{ position: "relative", height: 420, background: "var(--bg)" }}>
      <div style={{ position: "absolute", top: -8, left: 0, right: 0, height: 0 }}>
        <MissionSide over open>
          <MissionSideFacts>
            <MissionSideFact label="State" hideLabel kind="state">
              <Status s={{ icon: "spinner", tone: "progress", words: "Active" }} />
            </MissionSideFact>
            <MissionSideFact label="Slug">
              <Copyable text="export-invoices-as-csv" icon="over" />
            </MissionSideFact>
          </MissionSideFacts>
        </MissionSide>
      </div>
    </div>
  ),
};

const caps = takenMissions.flatMap((m) => m.d.capsules);
const withPR = caps.find((c) => c.pr);

// Its parts: what's known (a state with what's done to it, where it's
// at, its names, where it came from), capsules (selected, with changes
// not committed, burned), the orchestrator's runs, and the way to more.
export const Parts: StoryObj = {
  render: () => (
    <div style={{ display: "flex", gap: 32, alignItems: "start", padding: 16, fontSize: "var(--text-sm)", background: "var(--bg)" }}>
      {column(
        <MissionSideFacts>
          <MissionSideFact label="State" hideLabel kind="state">
            <Status s={{ icon: "pause", tone: "quiet", words: "Parked" }} />
            <Button variant="ghost" size="small">
              <Icon name="play" size={12} /> Unpark
            </Button>
          </MissionSideFact>
          <MissionSideFact label="At" kind="at">
            step 3, Review
          </MissionSideFact>
          <MissionSideFact label="Created">
            <Time iso="2026-11-20T09:12:00Z" />
          </MissionSideFact>
          <MissionSideFact label="About" kind="about">
            <span className="mono">PAY-2041</span>
            <span className="mono">api#41</span>
          </MissionSideFact>
          <MissionSideFact label="Raised from" mono>
            PAY-2041
          </MissionSideFact>
          <MissionSideFact label="Came from">
            <MissionSideCameFrom to="/conversations/w1">Exports for the accountants</MissionSideCameFrom>
          </MissionSideFact>
        </MissionSideFacts>,
      )}
      {column(
        <MissionSideSection title="Capsules" count={4}>
          <MissionSideCapsule id="a" name="api/export-csv" title="Review it (v)" onReview={none} />
          <MissionSideCapsule id="b" name="web/export-csv" title="Review it (v)" onReview={none} selected dirty />
          {withPR && <MissionSideCapsule id="c" name={`${withPR.repo}/${withPR.name}`} title="Review it (v)" onReview={none} badges={<MissionCapsuleBadges c={{ ...withPR, dirty: false }} />} />}
          <MissionSideCapsule id="d" name="api/old-try" title="Burned: its folder's gone" onReview={none} gone badges={<MissionCapsuleBadges c={{ repo: "api", name: "old-try", exists: false } as never} />} />
        </MissionSideSection>,
      )}
      {column(
        <MissionSideSection title="Orchestrator" count={3}>
          <MissionSideRun to="/" status={<Status s={{ icon: "check-circle", tone: "done", words: "Acted" }} />} what="step 2 done (c1)" iso="2026-11-25T10:20:00Z" />
          <MissionSideRun to="/" status={<Status s={{ icon: "circle-dashed", tone: "quiet", words: "Did nothing" }} />} what="a long reason it was run for, cut to the column" iso="2026-11-25T09:20:00Z" />
          <MissionSideRun to="/" status={<Status s={{ icon: "x-circle", tone: "danger", words: "Failed" }} />} what="c2 asked" iso="2026-11-24T09:20:00Z" />
          <MissionSideMore onClick={none}>3 more in Work</MissionSideMore>
        </MissionSideSection>,
      )}
    </div>
  ),
};
