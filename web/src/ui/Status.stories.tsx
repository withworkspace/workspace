import type { Meta, StoryObj } from "@storybook/react-vite";
import { Status, type StatusInfo } from "./Status";
import { StatusIcon } from "./StatusIcon";
import { Text } from "./Text";

// A status: where something stands, in each tone, with the ones going on
// now (their icon pulses, or turns); the icon alone, as a list's row has
// it; and a StatusIcon, an icon in a tone with no words, for a kind of
// thing.

const meta: Meta = { title: "Components/Status" };
export default meta;

const each: StatusInfo[] = [
  { icon: "question", tone: "attention", words: "Asks you" },
  { icon: "circle-dot", tone: "progress", words: "Working", live: true },
  { icon: "spinner", tone: "progress", words: "Starting", live: true },
  { icon: "spinner", tone: "waiting", words: "Checks running", live: true },
  { icon: "pr", tone: "waiting", words: "Awaiting review" },
  { icon: "check-circle", tone: "done", words: "Landed" },
  { icon: "x-circle", tone: "danger", words: "Checks failing" },
  { icon: "lock", tone: "danger", words: "Needs permission" },
  { icon: "pause", tone: "quiet", words: "Parked" },
];

export const Tones: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 10, justifyItems: "start" }}>
      {each.map((s) => (
        <Status key={s.words} s={s} />
      ))}
    </div>
  ),
};

export const IconOnly: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
        {each.map((s) => (
          <Status key={s.words} s={s} iconOnly />
        ))}
      </div>
      <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
        {each.map((s) => (
          <Status key={s.words} s={s} iconOnly size={13} title={`${s.words}, at 13`} />
        ))}
      </div>
      <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
        <StatusIcon icon="message" tone="quiet" />
        <StatusIcon icon="reply" tone="attention" title="Waiting on you" />
        <StatusIcon icon="circle-dot" tone="progress" live size={20} />
        <Text dim>StatusIcon: a tone's icon that says nothing</Text>
      </div>
    </div>
  ),
};
