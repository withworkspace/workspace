import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { useLocation } from "react-router";
import { Arrange } from "../shell/Arrange/Arrange";
import { arrange, type Place, placesOf } from "../shell/places";
import { Button } from "../ui/Button";
import { FindButton } from "./FindButton";
import { Foot } from "./Foot";
import { FootSidebar } from "./FootSidebar";
import { type Need, useArrived } from "./need";
import { Needs } from "./Needs";
import { Places } from "./Places";
import { SamplePins } from "./SamplePins";
import {
  crewAsks,
  manyToReview,
  needs,
  news,
  placesFor,
  toReview,
  workspace,
} from "./samples";
import { Sidebar } from "./Sidebar";
import { WorkspaceName } from "./WorkspaceName";

// The sidebar: the workspace's name, Find, its places and what needs you.
// It's drawn open or folded to a rail, which opens over the page while
// it's pointed at. Hidden, it isn't drawn at all (Chrome/Shell has that).
// The places and needs are Sorrel's, worked out as the app works them
// out; press a place and the page you're on moves with it. Focus, your
// pins, is between the two when you have any.

const meta: Meta = { title: "Chrome/Sidebar" };
export default meta;

const ground: React.CSSProperties = {
  display: "flex",
  gap: 24,
  alignItems: "stretch",
  height: 560,
  padding: 12,
  background: "var(--bg)",
};

type CardProps = {
  rail?: boolean;
  now?: Need[];
  fresh?: Set<string>;
  marks?: Record<string, string>;
  places?: Place[];
  wants?: string;
  pinned?: boolean;
  frozen?: { dragging: string; before: string };
};

// The sidebar's card, with all that's in it.
function Card({
  rail,
  now = needs,
  fresh,
  marks,
  places,
  wants,
  pinned,
  frozen,
}: CardProps) {
  const { pathname } = useLocation();
  return (
    <Sidebar rail={rail}>
      <WorkspaceName name="Sorrel" root="/try/sorrel" wants={wants} />
      <FindButton />
      <Places places={places ?? placesFor(now)} path={pathname} news={marks} />
      {pinned && <SamplePins path={pathname} frozen={frozen} />}
      <Needs needs={now} path={pathname} fresh={fresh} />
    </Sidebar>
  );
}

export const Open: StoryObj = {
  render: () => (
    <div style={ground}>
      <Card />
    </div>
  ),
};

// Folded, on a page that wants the room. Point at it, or tab into it, and
// it opens over the page.
export const Rail: StoryObj = {
  render: () => (
    <div style={ground}>
      <Card rail />
      <p className="dim" style={{ marginLeft: 48, maxWidth: 320 }}>
        The page. The rail opens over it while it's pointed at, and a count is a
        dot on its place's icon.
      </p>
    </div>
  ),
};

// A need that arrives while you're looking: its row lights up once where
// it is, open and in the rail, the counts change, and that's all. With
// the sidebar hidden it's its toggle in the foot, which says how many, that
// lights up.
function Arrives() {
  const [asked, setAsked] = useState(false);
  const now = asked ? [crewAsks, ...needs] : needs;
  const fresh = useArrived(now);
  return (
    <div style={{ background: "var(--bg)" }}>
      <div style={{ display: "flex", gap: 8, padding: "12px 12px 0" }}>
        <Button size="small" onClick={() => setAsked(true)} disabled={asked}>
          A crew asks
        </Button>
        <Button
          size="small"
          variant="ghost"
          onClick={() => setAsked(false)}
          disabled={!asked}
        >
          It's answered
        </Button>
      </div>
      <div style={{ ...ground, height: 520 }}>
        <Card now={now} fresh={fresh} />
        <Card rail now={now} fresh={fresh} />
      </div>
      <Foot
        start={
          <FootSidebar
            shown={false}
            count={now.length}
            fresh={fresh.size > 0}
          />
        }
      />
    </div>
  );
}

export const ANeedArrives: StoryObj = { render: () => <Arrives /> };

// Something new at a place since you last looked: a mark, in the accent.
// A place with a count has no mark: what needs you says more.
export const WithNews: StoryObj = {
  render: () => (
    <div style={ground}>
      <Card marks={news} />
      <Card rail marks={news} />
    </div>
  ),
};

// A plugin's views under its group's title, and a place moved into the
// workspace's menu that wants you: a dot on the name.
const board: Place[] = [
  ...placesFor(needs),
  {
    id: "x/jira/board",
    to: "/x/jira/board",
    icon: "list-checks",
    label: "Board",
    under: "Jira",
    count: 2,
    countTitle: "to see",
    match: (p) => p.startsWith("/x/jira/board"),
  },
  {
    id: "x/jira/standup",
    to: "/x/jira/standup",
    icon: "user",
    label: "Standup",
    under: "Jira",
    match: (p) => p.startsWith("/x/jira/standup"),
  },
];

export const APluginsPlaces: StoryObj = {
  render: () => (
    <div style={ground}>
      <Card places={board} wants="Runs: 1 failed" />
      <Card rail places={board} wants="Runs: 1 failed" />
    </div>
  ),
};

// Pull requests that ask for your review by name: under a small heading
// of their own, after what's yours. More than a few, and the rest are a
// press away in Reviews; with nothing of yours, they're all there is. In
// the rail the heading is a short rule.
export const ToReview: StoryObj = {
  render: () => (
    <div style={{ ...ground, height: 720 }}>
      <Card now={[...needs, ...toReview]} />
      <Card now={[...needs.slice(0, 2), ...manyToReview]} />
      <Card now={toReview.slice(0, 2)} />
      <Card rail now={[...needs, ...toReview]} />
    </div>
  ),
};

// Nothing needs you: the section isn't there.
export const NothingNeedsYou: StoryObj = {
  render: () => (
    <div style={ground}>
      <Card now={[]} />
      <Card rail now={[]} />
    </div>
  ),
};

// Focus: your pins, between the places and what needs you, drawn as Needs
// you is. Point at one for its ✕; drag one to put them in order. In the
// rail they're their icons, under a rule of their own.
export const WithFocus: StoryObj = {
  render: () => (
    <div style={{ ...ground, height: 680 }}>
      <Card pinned />
      <Card rail pinned />
    </div>
  ),
};

// A pin mid-drag: it's faint where it was, and a line says where it'd go.
export const PuttingPinsInOrder: StoryObj = {
  render: () => (
    <div style={{ ...ground, height: 680 }}>
      <Card
        pinned
        frozen={{
          dragging: "records:adr/0003",
          before: "dark-mode-for-the-dashboard",
        }}
      />
    </div>
  ),
};

// Arranging the sidebar: its places as rows with a grip, and under them
// those in the workspace's menu. It takes the places' room, and Focus and
// Needs you step aside until it's done. It's the app's own part
// (shell/Arrange/Arrange.tsx), in the new card as it is.
function Arranging() {
  const [done, setDone] = useState<string>();
  const start = arrange(placesOf(workspace, []));
  return (
    <div style={{ ...ground, height: 680 }}>
      <Sidebar>
        <WorkspaceName name="Sorrel" root="/try/sorrel" />
        <FindButton />
        <Arrange
          start={start}
          canReset
          onDone={(a) =>
            setDone(
              `Done: ${a.sidebar.map((p) => p.label).join(", ")}; in the menu: ${a.menu.map((p) => p.label).join(", ") || "nothing"}`,
            )
          }
          onCancel={() => setDone("Cancelled")}
          onReset={() => setDone("Reset")}
        />
      </Sidebar>
      {done && (
        <p className="dim" style={{ maxWidth: 320 }}>
          {done}
        </p>
      )}
    </div>
  );
}

export const ArrangingIt: StoryObj = { render: () => <Arranging /> };
