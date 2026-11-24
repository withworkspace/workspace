import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { billing, crew, details, freezes, many, rounding, type SampleThing } from "./dockSamples";
import { SampleBelow, useBelowStarted } from "./SampleBelow";
import { SampleBeside } from "./SampleBeside";

// A dock beside the page: a peek at a second thing while the page stays
// where it is, one up from a hover card. Its tabs (the things open in
// it), its tools (float it, close it), its edge to drag (or to move with
// the arrow keys, once it has them; a double-click puts it back), and
// what a tab shows, with a place to say something at its foot where the
// thing can be said something to.
//
// Next to the page, the page makes room; over it, the page keeps its
// width and the dock floats over its end. With more tabs than fit, they
// give up room, then the row scrolls (a mouse's wheel turns it along),
// and the list of them all is at its end.
//
// Below the page it's the same card along the bottom, for what's going
// on: its tabs are views, not things you opened (but for a run's
// terminal), and its head says which of each is shown.

const meta: Meta = { title: "Chrome/Dock" };
export default meta;

const frame: React.CSSProperties = { position: "relative", display: "flex", height: 520, background: "var(--bg)" };
const page: React.CSSProperties = { flex: 1, minWidth: 0, padding: 24, color: "var(--dim)" };

function Peek({ things, own, floats = false, width = 420, peek }: { things: SampleThing[]; own?: SampleThing[]; floats?: boolean; width?: number; peek?: string }) {
  const [over, setOver] = useState(floats);
  const [size, setSize] = useState(width);
  const [open, setOpen] = useState(true);
  return (
    <div style={frame}>
      <div style={page}>
        <p>The page. {over ? "It keeps its width: the dock is over it." : "It makes room for the dock."}</p>
        {!open && (
          <Button size="small" onClick={() => setOpen(true)}>
            Open it again
          </Button>
        )}
      </div>
      {open && (
        <div style={over ? { position: "absolute", top: 12, right: 12, bottom: 12, display: "flex" } : { display: "flex", margin: "12px 12px 12px 0" }}>
          <SampleBeside things={things} own={own} over={over} onOver={setOver} size={size} onSize={setSize} onClose={() => setOpen(false)} peek={peek} />
        </div>
      )}
    </div>
  );
}

// Next to the page: a conversation in front, with a place to reply, and a
// document and a crew behind it.
export const BesideThePage: StoryObj = { render: () => <Peek things={[rounding, billing, crew]} /> };

// Over the page, a little see-through, with what's under it blurred.
export const OverThePage: StoryObj = { render: () => <Peek things={[billing, rounding]} floats /> };

// One thing, which is most peeks.
export const OneThing: StoryObj = { render: () => <Peek things={[freezes]} /> };

// One of them a peek (z, on what the pointer's on): its name is slanted,
// and the next thing peeked at takes its place.
export const APeekAmongYours: StoryObj = { render: () => <Peek things={[billing, rounding]} peek="billing" /> };

// More than fit: the tabs behind give up room, the row scrolls, and the
// list of them all is at its end.
export const ManyTabs: StoryObj = { render: () => <Peek things={many} /> };

// The page's own tab first (it has no way to close it), a line, then
// yours.
export const ThePagesOwnFirst: StoryObj = { render: () => <Peek things={[rounding, crew]} own={[details]} /> };

// As narrow as it goes.
export const Narrowest: StoryObj = { render: () => <Peek things={[rounding, billing, crew]} width={300} /> };

function Low() {
  const started = useBelowStarted(true);
  if (!started) return null;
  return (
    <div style={{ ...frame, flexDirection: "column" }}>
      <div style={page}>
        <p>The page. It makes room for the dock.</p>
      </div>
      <div style={{ display: "flex", flexDirection: "column", margin: "0 12px 12px" }}>
        <SampleBelow />
      </div>
    </div>
  );
}

// Below the page: what's running and what it came to, here's or
// everywhere's, and a run's terminal you opened. Its edge is its top.
export const BelowThePage: StoryObj = { render: () => <Low /> };
