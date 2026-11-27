import { missionKeys } from "../keys";
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { RefsProvider } from "../../refs/known";
import { storyWorkspace, takenMissions } from "../../stories/missions";
import { Count } from "../../ui/Count";
import { PageBar, pageBarTabsClass } from "../../ui/PageBar";
import { Skeleton } from "../../ui/Skeleton";
import { Status } from "../../ui/Status";
import { headlineStatus } from "../../ui/statusOf";
import { Tab, Tabs } from "../../ui/Tabs";
import { Log } from "../Log/Log";
import { MissionData } from "../MissionData";
import { MissionTimeline } from "../MissionTimeline";
import { Side } from "../Side/Side";
import { Summary } from "../Summary/Summary";
import { useTimeline } from "../timeline";
import { MissionFrame, MissionFrameBody, MissionFrameMain, MissionFramePanel, missionFrameBarClass } from "./MissionFrame";
import { Text } from "../../ui/Text";

// A mission's page as its frame draws it: the timeline and the bar, then
// the tab under them, in a column with the details beside it or alone,
// with the details over another tab, and while the mission loads. Each is
// a mission ws gave (mission/fixtures), drawn by the page's own parts.

const meta: Meta = { title: "Mission/MissionFrame" };
export default meta;

const none = () => {};
const tabs = ["Summary", "Brief", "Work", "Verify", "Log", "Talk", "Decisions"];

function Bar({ title, status, under, at = "Summary" }: { title?: string; status?: ReactNode; under?: ReactNode; at?: string }) {
  return (
    <PageBar
      className={missionFrameBarClass}
      back={{ to: "/", title: "Missions", keys: missionKeys.back, label: "Back to the missions" }}
      status={status}
      title={title}
      tabs={
        <Tabs className={pageBarTabsClass()} value={at} onChange={none} controls="mission-panel" aria-label="Mission">
          {tabs.map((t) => (
            <Tab key={t} value={t}>
              {t}
              {t === "Verify" && <Count tone="attention">1</Count>}
            </Tab>
          ))}
        </Tabs>
      }
      under={under}
    />
  );
}

// The page, in a box the window's height would give it.
const box = (children: ReactNode) => <div style={{ height: 720, overflow: "hidden", border: "1px solid var(--line)", background: "var(--bg)" }}>{children}</div>;

function Taken({ name, side, over }: { name: string; side?: boolean; over?: boolean }) {
  storyWorkspace();
  const m = takenMissions.find((x) => x.name === name);
  const strip = useTimeline(m?.d);
  if (!m) return <Text as="p" dim>No mission {name} among the fixtures.</Text>;
  const d = m.d;
  const details = (overTab?: boolean) => <Side d={d} selected={() => false} openArtifact={none} review={none} toWork={none} reload={async () => {}} over={overTab} open={overTab} />;
  return (
    <RefsProvider>
      <MissionData.Provider value={d}>
        {box(
          <MissionFrame>
            {strip && <MissionTimeline {...strip} />}
            <Bar title={d.title} status={d.headlines[0] && <Status s={headlineStatus(d.headlines[0])} iconOnly size={16} />} at={over ? "Log" : "Summary"} under={over && details(true)} />
            <MissionFrameBody side={side}>
              <MissionFrameMain>
                <MissionFramePanel id="mission-panel" label={over ? "Log" : "Summary"}>
                  {over ? <Log d={d} selected={() => false} openArtifact={none} toWork={none} /> : <Summary d={d} selected={() => false} review={none} full={!side} go={none} reload={async () => {}} />}
                </MissionFramePanel>
              </MissionFrameMain>
              {side && details()}
            </MissionFrameBody>
          </MissionFrame>,
        )}
      </MissionData.Provider>
    </RefsProvider>
  );
}

// Summary with the details beside it.
export const WithDetails: StoryObj = { render: () => <Taken name="export-invoices-as-csv" side /> };

// Summary alone: what the details had is in it.
export const Alone: StoryObj = { render: () => <Taken name="export-invoices-as-csv" /> };

// Another tab (Log), with the details slid in over it.
export const DetailsOver: StoryObj = { render: () => <Taken name="reminders-for-overdue-invoices" over /> };

// While the mission loads: the bar, with what the list said of it, over
// where the tab will be.
export const Loading: StoryObj = {
  render: () =>
    box(
      <MissionFrame>
        <MissionTimeline />
        <Bar title="Export invoices as CSV" />
        <MissionFrameBody>
          <MissionFrameMain>
            <Skeleton rows={6} />
          </MissionFrameMain>
        </MissionFrameBody>
      </MissionFrame>,
    ),
};
