import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { Segment, Segmented, type SegmentedLook, segmentClass, segmentedClass } from "./Segmented";

// A segmented control: as it is, small, filling what it's in, with icons
// and counts, disabled, and its look alone on something that isn't one.

const meta: Meta = { title: "Components/Segmented" };
export default meta;

const row = { display: "flex", gap: 16, alignItems: "center", flexWrap: "wrap" } as const;

function Widths({ disabled, ...look }: SegmentedLook & { disabled?: boolean }) {
  const [width, setWidth] = useState("fit");
  return (
    <Segmented {...look} value={width} onChange={setWidth} disabled={disabled} aria-label="Width">
      <Segment value="fit" title="As wide as the page">
        Fit
      </Segment>
      <Segment value="1280" title="A laptop">
        1280
      </Segment>
      <Segment value="768" title="A tablet">
        768
      </Segment>
      <Segment value="390" title="A phone">
        390
      </Segment>
    </Segmented>
  );
}

function Layout() {
  const [layout, setLayout] = useState("unified");
  return (
    <Segmented value={layout} onChange={setLayout} aria-label="Diff layout">
      <Segment value="unified" title="Unified (u)">
        <Icon name="unified" size={13} /> Unified
      </Segment>
      <Segment value="split" title="Side by side (u)">
        <Icon name="split" size={13} /> Split
      </Segment>
    </Segmented>
  );
}

function Show() {
  const [show, setShow] = useState("changed");
  return (
    <Segmented fill value={show} onChange={setShow} aria-label="Show">
      <Segment value="changed">
        Changed <Count>4</Count>
      </Segment>
      <Segment value="all">
        All <Count>212</Count>
      </Segment>
      <Segment value="none" disabled>
        Ignored <Count>0</Count>
      </Segment>
    </Segmented>
  );
}

export const Looks: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 20 }}>
      <div style={row}>
        <span className="dim" style={{ width: 80 }}>
          Plain
        </span>
        <Widths />
        <Layout />
      </div>
      <div style={row}>
        <span className="dim" style={{ width: 80 }}>
          Small
        </span>
        <Widths size="small" />
      </div>
      <div style={row}>
        <span className="dim" style={{ width: 80 }}>
          Disabled
        </span>
        <Widths disabled />
      </div>
      <div style={row}>
        <span className="dim" style={{ width: 80 }}>
          Fill
        </span>
        <div style={{ width: 360 }}>
          <Show />
        </div>
      </div>
    </div>
  ),
};

// The look on its own, for a track whose segments something else draws.
export const LookAlone: StoryObj = {
  render: () => (
    <div className="page" style={row}>
      <div className={segmentedClass()} role="group" aria-label="Versions">
        {[1, 2, 3].map((n) => (
          <button key={n} type="button" data-ui="" className={segmentClass()} aria-pressed={n === 2} data-selected={n === 2 || undefined}>
            v{n}
          </button>
        ))}
      </div>
    </div>
  ),
};
