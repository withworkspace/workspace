import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { Keys } from "./kit";
import { ToggleButton, type ToggleLook } from "./ToggleButton";

// Toggle buttons: off and on, plain and ghost, small, square (an icon
// alone), disabled either way, and one whose page gives it a variant
// while it's on.

const meta: Meta = { title: "Components/ToggleButton" };
export default meta;

const row = { display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" } as const;

function Toggle({ start = false, children, ...look }: ToggleLook & { start?: boolean; disabled?: boolean; children: React.ReactNode }) {
  const [on, setOn] = useState(start);
  return (
    <ToggleButton {...look} pressed={on} onChange={setOn}>
      {children}
    </ToggleButton>
  );
}

const looks: { name: string; look: ToggleLook }[] = [
  { name: "Plain", look: {} },
  { name: "Ghost", look: { variant: "ghost" } },
];

export const Looks: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16 }}>
      {looks.map(({ name, look }) => (
        <div key={name} style={row}>
          <span className="dim" style={{ width: 80 }}>
            {name}
          </span>
          <Toggle {...look}>
            <Icon name="panel" size={12} /> Details
          </Toggle>
          <Toggle {...look} start>
            <Icon name="panel" size={12} /> Details
          </Toggle>
          <Toggle {...look} size="small" start>
            <Icon name="eye" size={13} /> Worth knowing <Count tone="attention">3</Count>
          </Toggle>
          <Toggle {...look} size="small">
            <Icon name="code" size={13} /> Source <Keys k="s" />
          </Toggle>
          <Toggle {...look} square start>
            <Icon name="pin" size={13} />
          </Toggle>
          <Toggle {...look} disabled>
            Disabled
          </Toggle>
          <Toggle {...look} disabled start>
            Disabled, on
          </Toggle>
        </div>
      ))}
    </div>
  ),
};

function NotRight() {
  const [on, setOn] = useState(true);
  return (
    <ToggleButton variant={on ? "danger" : undefined} pressed={on} onChange={setOn}>
      <Icon name="x" size={14} /> Not right <Keys k="n" />
    </ToggleButton>
  );
}

// A page says what on means where the accent doesn't: the variant it has
// while it's on, here danger.
export const AVariantWhileOn: StoryObj = {
  render: () => (
    <div className="page" style={row}>
      <NotRight />
    </div>
  ),
};
