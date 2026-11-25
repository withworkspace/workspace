import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { TeamTopic } from "../api";
import { TeamSummary, type TeamSummaryData } from "./TeamSummary";

// The Team page's account of a period: written and current, written and
// changed since, with a topic's changes alone showing, being written,
// couldn't be written, and folded.

const meta: Meta = { title: "Team/TeamSummary" };
export default meta;

const people: Record<string, string> = { james: "James", claude: "Claude", ana: "Ana" };
const names = (keys: string[]) => keys.map((k) => people[k] ?? k);
const ago = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();

const topics: TeamTopic[] = [
  {
    name: "Plugin framework and plugin tooling",
    text: "The framework moved to `github.com/withworkspace/plugin`, and releases publish from tags that must match `plugin.toml`. Plugins importing the old path must change their import and `go.mod`. The plugin-dev tool and the missing-features doc have landed.",
    strands: ["a", "b", "c", "d"],
    people: ["james", "claude"],
  },
  {
    name: "ws initial codebase and conventions",
    text: "All landed. The repo must stay public, because the curl installer no longer logs in. The web UI uses pnpm with a committed lockfile, and CI runs with `--frozen-lockfile`. Commits and PR titles follow Conventional Commits.",
    strands: ["e", "f", "g", "h"],
    people: ["james"],
  },
  {
    name: "Customisable UI accent",
    text: "The design doc has landed. One `[ui] accent` setting in `ws.local.toml` picks the colour, and the UI's accent colours are derived from it.",
    strands: ["i"],
    people: ["james", "ana"],
  },
];

function Live({ summary, folded: startFolded = false, on: startOn }: { summary: TeamSummaryData; folded?: boolean; on?: string }) {
  const [folded, setFolded] = useState(startFolded);
  const [on, setOn] = useState(startOn);
  return (
    <div className="page" style={{ maxWidth: 960 }}>
      <TeamSummary summary={summary} folded={folded} onFold={() => setFolded(!folded)} on={on} onTopic={(t) => setOn(on === t.name ? undefined : t.name)} names={names} onWrite={() => {}} />
    </div>
  );
}

export const Written: StoryObj = { render: () => <Live summary={{ at: ago(0.2), topics, can: 9 }} /> };

export const ChangedSince: StoryObj = { render: () => <Live summary={{ at: ago(8), topics, stale: true, can: 12 }} /> };

export const OneTopic: StoryObj = { render: () => <Live summary={{ at: ago(1), topics, can: 9 }} on={topics[1]!.name} /> };

export const Writing: StoryObj = { render: () => <Live summary={{ at: ago(8), topics, writing: true, can: 12 }} /> };

export const Failed: StoryObj = { render: () => <Live summary={{ failed: "the agent exited: rate limited", can: 4 }} /> };

export const Folded: StoryObj = { render: () => <Live summary={{ at: ago(1), topics, can: 9 }} folded /> };
