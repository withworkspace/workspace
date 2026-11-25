import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Artifact } from "../api";
import { MissionOutput } from "./MissionOutput";
import { MissionOutputState } from "./MissionOutputState";
import { MissionOutputThreads } from "./MissionOutputThreads";
import { MissionSideSection } from "./MissionSideSection";

// A mission's outputs: as its sidebar lists them (compact, under a
// section's head), as rows with what's known of each, and where each
// can stand.

const meta: Meta = { title: "Mission/MissionOutputs" };
export default meta;

const added = "2026-11-24T10:00:00Z";
const outputs: Artifact[] = [
  { ref: "docs/plans/plugin-pages.md", kind: "doc", label: "Plan: Plugin pages as single-file bundles", state: "landed", by: "c1", added, readable: true },
  { ref: "c1:handoff", kind: "handoff", label: "Design written as a Plan: the shell, the bundle and how ws serves it.", state: "private", by: "c1", added, readable: true },
  { ref: "c2:handoff", kind: "handoff", label: "UI lib built: the client half, its types and a story for each part.", state: "private", by: "c2", added, readable: true, threads: { open: 2, waiting: 1 } },
  { ref: "a3", kind: "artifact", label: "Plugin page preview", state: "live", by: "c3", added, readable: false, versions: 3 },
  { ref: "https://github.com/acme/app/pull/42", kind: "pr", label: "Serve plugin pages from /plugins", state: "open", by: "c4", added, readable: false, url: "https://github.com/acme/app/pull/42" },
];

const open = () => {};

export const Sidebar: StoryObj = {
  render: () => (
    <div className="page" style={{ width: 280 }}>
      <MissionSideSection title="Outputs" count={outputs.length}>
        {outputs.map((a, i) => (
          <MissionOutput key={a.ref} a={a} compact selected={i === 1} open={open} />
        ))}
      </MissionSideSection>
    </div>
  ),
};

export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 560 }}>
      {outputs.map((a) => (
        <MissionOutput key={a.ref} a={a} open={open} />
      ))}
    </div>
  ),
};

export const States: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
      {["draft", "private", "open", "in review", "approved", "landed", "merged", "missing", "live", "promoted", "discarded"].map((s) => (
        <MissionOutputState key={s} state={s} />
      ))}
      <MissionOutputThreads t={{ open: 3, waiting: 0 }} />
      <MissionOutputThreads t={{ open: 2, waiting: 1 }} />
    </div>
  ),
};
