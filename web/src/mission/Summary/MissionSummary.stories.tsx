import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { MoreLink } from "../../ui/MoreLink";
import { Summary as SummaryList, SummaryRow } from "../../ui/Summary";
import { MissionCapsuleBadges } from "../MissionCapsuleBadges";
import { MissionData } from "../MissionData";
import { MissionSummaryCapsule } from "./MissionSummaryCapsule";
import { MissionSummaryMore } from "./MissionSummaryMore";
import { MissionSummarySaid } from "./MissionSummarySaid";
import { MissionSummaryWork } from "./MissionSummaryWork";
import { Summary } from "./Summary";
import { Text } from "../../ui/Text";

// A mission's Summary tab: as missions ws gave (mission/fixtures) have it,
// beside its details and alone (full, with what the details had), and its
// parts' states drawn from props.

const meta: Meta = { title: "Mission/MissionSummary" };
export default meta;

const none = () => {};

// The tab's column, as the page gives it.
const column = (children: ReactNode) => <div style={{ maxWidth: 916, padding: "16px 32px 48px", background: "var(--bg)" }}>{children}</div>;

function Taken({ name, full }: { name: string; full?: boolean }) {
  storyWorkspace();
  const m = takenMissions.find((x) => x.name === name);
  if (!m) return <Text as="p" dim>No mission {name} among the fixtures.</Text>;
  return (
    <RefsProvider>
      <MissionData.Provider value={m.d}>
        {column(<Summary d={m.d} selected={(k) => k === `c:${m.d.capsules[0]?.repo}/${m.d.capsules[0]?.name}`} review={none} full={!!full} go={none} reload={async () => {}} />)}
      </MissionData.Provider>
    </RefsProvider>
  );
}

// Beside its details: the goal, what's been decided and what happened.
export const BesideDetails: StoryObj = { render: () => <Taken name="export-invoices-as-csv" /> };

// Alone: its plan, its work (the first capsule selected), its crew too.
export const Alone: StoryObj = { render: () => <Taken name="export-invoices-as-csv" full /> };

// Alone, with work in three repos and crew to hear from.
export const AcrossRepos: StoryObj = { render: () => <Taken name="work-across-repos" full /> };

// With no headline to lead with: the step it's on, and what crew said, as
// rows of their own.
export const Ended: StoryObj = { render: () => <Taken name="which-index-is-missing" full /> };

const caps = takenMissions.flatMap((m) => m.d.capsules);
const withPR = caps.find((c) => c.pr) ?? caps[0];
const said = takenMissions.flatMap((m) => m.d.crew).find((c) => c.lastReply)?.lastReply ?? "Done: the export has a header row.";

// Its parts: capsules in a line (selected, on a branch of another name,
// burned), what's said of Work under them, crew's last word, and the way
// to more.
export const Parts: StoryObj = {
  render: () =>
    column(
      <SummaryList>
        <SummaryRow label="Work">
          <MissionSummaryCapsule id="a" name="api/export-csv" badges={<MissionCapsuleBadges c={{ repo: "api", name: "export-csv", exists: true, ahead: 3 } as never} />} onReview={none} />
          <MissionSummaryCapsule id="b" name={`${withPR.repo}/${withPR.name}`} branch="feat/export-csv" badges={<MissionCapsuleBadges c={withPR} />} onReview={none} selected />
          <MissionSummaryCapsule id="c" name="api/old-try" badges={<MissionCapsuleBadges c={{ repo: "api", name: "old-try", exists: false } as never} />} />
          <MissionSummaryWork afterCards>
            Done: web · 2 outputs{" "}
            <MoreLink onClick={none}>All of it in Work</MoreLink>
          </MissionSummaryWork>
        </SummaryRow>
        <SummaryRow label="Work">
          <MissionSummaryWork>Nothing yet: crew lift a capsule when they start. </MissionSummaryWork>
        </SummaryRow>
        <SummaryRow label="From c1">
          <MissionSummarySaid text={said} />
        </SummaryRow>
        <SummaryRow label="Recently">
          <MissionSummaryMore onClick={none}>The whole log (30)</MissionSummaryMore>
        </SummaryRow>
      </SummaryList>,
    ),
};
