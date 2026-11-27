import type { KeyLike } from "../../keymap/keymap";
import { tabKeys } from "../keys";
import type { ReactNode } from "react";
import type { Headline, MissionDetail } from "../../api";
import { Count } from "../../ui/Count";
import { PageBar, pageBarTabsClass } from "../../ui/PageBar";
import { Status } from "../../ui/Status";
import { headlineStatus } from "../../ui/statusOf";
import { Tab, Tabs } from "../../ui/Tabs";
import { waiting as bundleWaiting } from "../verifyRows";
import { missionFrameBarClass } from "./MissionFrame";

export const missionTabs = ["summary", "brief", "work", "verify", "log", "talk", "decisions"] as const;
export type MissionTab = (typeof missionTabs)[number];
export const missionTabNames: Record<MissionTab, string> = { summary: "Summary", brief: "Brief", work: "Work", verify: "Verify", log: "Log", talk: "Talk", decisions: "Decisions" };

type MissionBarProps = {
  // The mission, once it's loaded: where it stands, and what's waiting in
  // each tab.
  d?: MissionDetail;
  // While it loads, what the workspace's list already says of it.
  title?: string;
  headline?: Headline;
  // The tab lit, and choosing one. A page under the mission (a crew's)
  // lights the tab it's reached from, and choosing that one goes there.
  tab: MissionTab;
  onTab: (t: MissionTab) => void;
  // Under the mission: the tab lit is a way there, not where you are.
  under?: boolean;
  // The way back.
  back: { to: string; title: string; keys?: KeyLike; label?: string };
  // The tabs' keys (1–7), said on each: the mission's page binds them.
  keys?: boolean;
  onRename?: (to: string) => void;
  // What hangs from the bar (the details, over a tab).
  hangs?: ReactNode;
  // Beside its name (its pin).
  children?: ReactNode;
};

// MissionBar is a mission's bar, on its page and the pages under it:
// back, where it stands, its name, and its tabs with what's waiting in
// each, in the mission's frame (MissionFrame).
export function MissionBar({ d, title, headline, tab, onTab, under, back, keys, onRename, hangs, children }: MissionBarProps) {
  const h = d ? d.headlines[0] : headline;
  // Your turn in them, or something they proposed to confirm. Its
  // planner's is Brief's, where it's beside the brief and plan.
  const waitingTalks = d ? d.talks.filter((x) => x.id !== d.planner && (x.waiting || !!x.proposed)).length : 0;
  // What its planner has for you on Brief: what it proposed (a draft of
  // the brief and plan, a change to them, anything else), or your turn.
  const planning = d?.talks.find((x) => x.id === d.planner);
  const briefWaiting = planning?.proposed || (planning?.waiting ? 1 : 0);
  // Bundles waiting for you, and those nothing has taken the place of:
  // what Verify has to show.
  const waitingBundles = d ? d.bundles.filter(bundleWaiting).length : 0;
  const standing = d ? d.bundles.filter((b) => !b.replacedBy).length : 0;
  const count = (t: MissionTab) => (
    <>
      {t === "brief" && briefWaiting > 0 && (
        <Count tone="attention" title={planning?.proposed ? "Planner proposal to confirm" : "Planner waiting on you"}>
          {briefWaiting}
        </Count>
      )}
      {t === "verify" && waitingBundles > 0 && (
        <Count tone="attention" title="Bundles to approve">
          {waitingBundles}
        </Count>
      )}
      {t === "verify" && !waitingBundles && standing > 0 && <Count title="Verification bundles">{standing}</Count>}
      {t === "talk" && waitingTalks > 0 && (
        <Count tone="attention" title="Your turn, or something to confirm">
          {waitingTalks}
        </Count>
      )}
    </>
  );
  return (
    <PageBar
      className={missionFrameBarClass}
      back={back}
      status={h && <Status s={headlineStatus(h)} iconOnly size={16} />}
      title={d?.title ?? title}
      onRename={onRename}
      renameLabel="Rename mission"
      tabs={
        <Tabs className={pageBarTabsClass()} value={tab} onChange={onTab} controls={under ? undefined : "mission-panel"} aria-label="Mission">
          {missionTabs.map((t) => (
            <Tab key={t} value={t} title={missionTabNames[t]} keys={keys ? tabKeys[t] : undefined} onPress={under && t === tab ? () => onTab(t) : undefined}>
              {missionTabNames[t]}
              {d && count(t)}
            </Tab>
          ))}
        </Tabs>
      }
      under={hangs}
    >
      {children}
    </PageBar>
  );
}
