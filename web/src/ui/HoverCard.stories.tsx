import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef, useState } from "react";
import { Chip } from "./Chip";
import { FloatingCard } from "./FloatingCard";
import { HoverCard, HoverCardHead, HoverCardLine, HoverCardLinks, hoverCardLinkClass, HoverCardSaid, HoverCardStrong, HoverCardTitle } from "./HoverCard";
import { Icon } from "./Icon";

// Hover cards: what a card says, laid out with its parts (drawn open
// here, so they can be looked at); something to point at, which shows its
// card after a moment; and where a card goes: under what it's about,
// over it at the window's foot, and at a list's side.

const meta: Meta = { title: "Components/HoverCard" };
export default meta;

const says = (
  <>
    <HoverCardHead>
      <Icon name="file" size={13} />
      <HoverCardTitle>adr/0001-money-is-whole-minor-units.md</HoverCardTitle>
      <Chip tone="done">current</Chip>
    </HoverCardHead>
    <HoverCardLine>
      <HoverCardStrong>ADR 0001</HoverCardStrong>: money is whole minor units
    </HoverCardLine>
    <HoverCardLine dim>in records, changed 2 days ago</HoverCardLine>
    <HoverCardSaid>A rounding is a decision, written down where it's made.</HoverCardSaid>
    <HoverCardLinks>
      <a className={hoverCardLinkClass()} href="#open">
        Open it <Icon name="arrow-right" size={11} />
      </a>
      <a className={hoverCardLinkClass()} href="#files">
        In Files <Icon name="arrow-right" size={11} />
      </a>
    </HoverCardLinks>
  </>
);

// A card held open by what it's about, for a story to show where it goes.
function Open({ words, beside, style }: { words: string; beside?: boolean; style?: React.CSSProperties }) {
  const [el, setEl] = useState<HTMLElement | null>(null);
  const list = useRef<HTMLDivElement>(null);
  return (
    <div ref={list} style={{ width: 200, padding: 8, border: "1px dashed var(--line)", borderRadius: 8, ...style }}>
      <span ref={setEl} className="dim">
        {words}
      </span>
      {el && (
        <FloatingCard anchor={el} beside={beside ? list.current : undefined} onEnter={() => {}} onLeave={() => {}}>
          {says}
        </FloatingCard>
      )}
    </div>
  );
}

export const Says: StoryObj = {
  render: () => (
    <div className="page" style={{ minHeight: 220 }}>
      <Open words="What a card says" />
    </div>
  ),
};

export const Hovered: StoryObj = {
  render: () => (
    <div className="page" style={{ minHeight: 220 }}>
      <span className="dim">Point at </span>
      <HoverCard trigger={<Chip tone="quiet">adr/0001</Chip>}>{says}</HoverCard>
      <span className="dim"> for a moment, or give it the keys.</span>
    </div>
  ),
};

export const Places: StoryObj = {
  render: () => (
    <div className="page" style={{ position: "relative", height: 520 }}>
      <Open words="Under it" />
      <Open words="At a list's side" beside style={{ position: "absolute", right: 24, top: 120 }} />
      <Open words="Over it, at the window's foot" style={{ position: "absolute", left: 24, bottom: 0 }} />
    </div>
  ),
};
