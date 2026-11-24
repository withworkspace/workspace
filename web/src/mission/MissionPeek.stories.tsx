import type { Meta, StoryObj } from "@storybook/react-vite";
import type { MissionDetail } from "../api";
import { MissionPeek } from "./MissionPeek";
import { MissionPeekCapsule } from "./MissionPeekCapsule";
import { MissionPeekCrew } from "./MissionPeekCrew";
import { MissionPeekStand } from "./MissionPeekStand";
import { peekOf } from "./peek";

// A mission as it's read beside another page (z, on a link to it): what
// it's for, what it stands on, its plan, its crew and its capsules. First
// its parts each alone, in their states; then the missions ws gave
// (fixtures/), each as its peek has it, at the width of the dock.

const meta: Meta = { title: "Mission/MissionPeek" };
export default meta;

const pane: React.CSSProperties = { width: 388, padding: 16, border: "1px solid var(--line)", borderRadius: 10, background: "var(--raised)" };
const column: React.CSSProperties = { display: "grid", gap: 10, width: 388 };
const list: React.CSSProperties = { display: "grid", gap: 4, margin: 0, padding: 0, listStyle: "none" };

export const Stands: StoryObj = {
  render: () => (
    <div className="page" style={column}>
      <MissionPeekStand lead tone="attention" icon="question" title="c2 asks: which header says when to try again?" detail="`Retry-After` is the standard one, in seconds. `X-RateLimit-Reset` is what the old gateway sent, as a time. Clients in the wild read both, so I can send both, but the docs should name one." />
      <MissionPeekStand lead tone="progress" icon="circle-dot" title="c3 is working on step 3" nothing />
      <MissionPeekStand lead tone="danger" icon="x-circle" title="api#46's checks are failing" detail="`make test` failed in `gateway/limit_test.go`." />
      <MissionPeekStand lead tone="done" icon="check-circle" title="Landed" />
      <MissionPeekStand tone="attention" icon="pr" title="api#46 is ready to merge" detail="It's approved and its checks pass" />
    </div>
  ),
};

export const Crew: StoryObj = {
  render: () => (
    <div className="page" style={column}>
      <ul style={list}>
        <MissionPeekCrew id="c2" status={{ icon: "circle-dot", tone: "progress", words: "Working", live: true }} what="claude on opus" capsule="api/rate-limit" to="/missions/rate-limit-the-public-api/crew/c2" />
        <MissionPeekCrew id="c3" status={{ icon: "question", tone: "attention", words: "Asks you" }} what="codex" capsule="docs/limits-page-with-a-long-name" to="/missions/rate-limit-the-public-api/crew/c3" />
        <MissionPeekCrew id="c4" status={{ icon: "check-circle", tone: "done", words: "Reported done" }} what="claude on sonnet" to="/missions/rate-limit-the-public-api/crew/c4" />
      </ul>
    </div>
  ),
};

export const Capsules: StoryObj = {
  render: () => (
    <div className="page" style={column}>
      <ul style={list}>
        <MissionPeekCapsule name="api/rate-limit" pr={{ name: "api#46", to: "/reviews/api/46", status: { icon: "pr", tone: "quiet", words: "Open" } }} />
        <MissionPeekCapsule name="web/limit-banner" pr={{ name: "web#12", to: "/reviews/web/12", status: { icon: "x-circle", tone: "danger", words: "Checks failing" } }} />
        <MissionPeekCapsule name="docs/limits-page" />
        <MissionPeekCapsule name="api/spike-token-bucket" gone pr={{ name: "api#41", to: "/reviews/api/41", status: { icon: "check-circle", tone: "done", words: "Merged" } }} />
      </ul>
    </div>
  ),
};

// One written out: a mission mid-plan, asking you something, with two
// crew and two capsules.
export const Whole: StoryObj = {
  render: () => (
    <div className="page">
      <div style={pane}>
        <MissionPeek
          goal="One customer's script can't slow the api for everyone else."
          stands={[
            { tone: "attention", icon: "question", title: "c2 asks: which header says when to try again?", detail: "`Retry-After` is the standard one, in seconds. Clients in the wild read both." },
            { tone: "progress", icon: "circle-dot", title: "c3 is working on step 3" },
          ]}
          steps={[
            { n: 1, title: "Limit each key", state: "accepted" },
            { n: 2, title: "Read it", state: "accepted" },
            { n: 3, title: "Say so in the docs", state: "current" },
            { n: 4, title: "Try it against staging", state: "todo" },
            { n: 5, title: "Land it", state: "todo" },
          ]}
          crew={[
            { id: "c2", status: { icon: "question", tone: "attention", words: "Asks you" }, what: "claude on opus", capsule: "api/rate-limit", to: "/missions/rate-limit-the-public-api/crew/c2" },
            { id: "c3", status: { icon: "circle-dot", tone: "progress", words: "Working", live: true }, what: "claude on sonnet", capsule: "docs/limits-page", to: "/missions/rate-limit-the-public-api/crew/c3" },
          ]}
          capsules={[{ name: "api/rate-limit", pr: { name: "api#46", to: "/reviews/api/46", status: { icon: "clock", tone: "waiting", words: "Checks running" } } }, { name: "docs/limits-page" }]}
        />
      </div>
    </div>
  ),
};

// Every mission ws gave, as its peek has it.
const missions = import.meta.glob<MissionDetail>("./fixtures/*/missions/*.json", { eager: true, import: "default" });

export const AsWsGaveThem: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", flexWrap: "wrap", gap: 20, alignItems: "start" }}>
      {Object.entries(missions).map(([path, d]) => (
        <div key={path} style={pane}>
          <h3 style={{ margin: "0 0 12px", fontSize: "var(--text-md)" }}>{d.title}</h3>
          <MissionPeek {...peekOf(d)} />
        </div>
      ))}
    </div>
  ),
};
