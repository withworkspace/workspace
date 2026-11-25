import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { editedDoc, editedDraft, picture } from "../samples";
import { DocEdit } from "./DocEdit";
import { MissionArtifactCode } from "./MissionArtifactCode";
import { MissionArtifactMedia } from "./MissionArtifactMedia";
import { MissionArtifactMeta } from "./MissionArtifactMeta";
import { MissionArtifactPath } from "./MissionArtifactPath";

// One of a mission's documents: a picture and a video crew handed over,
// the line over a document, a file as code, where it is on disk; and a
// document being edited, one that lands on its branch when it's saved
// (with an edit kept from before it changed), and the mission's own.

const meta: Meta = { title: "Mission/MissionArtifact" };
export default meta;

export const Media: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24, width: 900, padding: 16, background: "var(--bg)" }}>
      <MissionArtifactMedia src={picture} label="The list, sorted" />
      <MissionArtifactMedia src="" video label="mission:proof/list.webm" />
    </div>
  ),
};

export const Parts: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24, width: 900, padding: 16, background: "var(--bg)" }}>
      <MissionArtifactMeta
        kind="Proposal"
        hint="A change worth deciding on, before it's made"
        state="draft"
        by="c1"
        added={editedDraft.at}
        docRef={editedDoc.ref}
        actions={
          <Button size="small" title="Edit it here" keys="e">
            <Icon name="pencil" size={12} /> Edit <Keys k="e" />
          </Button>
        }
      />
      <MissionArtifactMeta kind="Handoff" state="landed" docRef="mission:handoffs/c1.md" />
      <MissionArtifactCode html={"func List() []Invoice {\n  return sorted(all)\n}"} />
      <MissionArtifactPath path="/tmp/ws/.ws/missions/reminders-for-overdue-invoices/handoffs/c1.md" />
    </div>
  ),
};

export const Edit: StoryObj = {
  render: () => (
    <div style={{ width: 900, padding: 16, background: "var(--bg)" }}>
      <DocEdit slug="reminders-for-overdue-invoices" a={editedDoc} draft={editedDraft} onSaved={() => {}} onClose={() => {}} />
    </div>
  ),
};

export const EditMission: StoryObj = {
  render: () => (
    <div style={{ width: 900, padding: 16, background: "var(--bg)" }}>
      <DocEdit slug="reminders-for-overdue-invoices" a={{ ...editedDoc, ref: "mission:brief.md", saves: undefined }} onSaved={() => {}} onClose={() => {}} />
    </div>
  ),
};
