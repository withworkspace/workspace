import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Foot } from "./Foot";
import { FootBelow } from "./FootBelow";
import { FootBeside } from "./FootBeside";
import { FootCrew } from "./FootCrew";
import { FootReady } from "./FootReady";
import { FootSends } from "./FootSends";
import { FootSep } from "./FootSep";
import { FootSidebar } from "./FootSidebar";
import { needs } from "./samples";

// The foot: with nothing going on, with crew at work, short of room (a
// mission's name is cut short, its crew's badge stays), and with the
// sidebar hidden, when it says how many need you.

const meta: Meta = { title: "Chrome/Foot" };
export default meta;

const Going = () => (
  <>
    <FootSep />
    <FootReady count={2} />
    <FootSep />
    <FootCrew id="c1" mission="Dark mode for the dashboard" doing="Editing Theme.tsx" to="/missions/dark-mode-for-the-dashboard/crew/c1" />
    <FootCrew id="c2" mission="Reminders for overdue invoices" doing="Running the tests" to="/missions/reminders-for-overdue-invoices/crew/c2" />
  </>
);

export const Quiet: StoryObj = {
  render: () => <Foot start={<FootSidebar shown count={0} />} />,
};

export const CrewAtWork: StoryObj = {
  render: () => (
    <Foot
      start={
        <>
          <FootSidebar shown count={0} />
          <Going />
        </>
      }
    />
  ),
};

// What's been said on pull requests and sent to GitHub behind: on its
// way, and what GitHub didn't take.
export const SendingToGitHub: StoryObj = {
  render: () => (
    <Foot
      start={
        <>
          <FootSidebar shown count={0} />
          <FootSep />
          <FootSends sending={3} />
        </>
      }
    />
  ),
};

export const DidntReachGitHub: StoryObj = {
  render: () => (
    <Foot
      start={
        <>
          <FootSidebar shown count={0} />
          <FootSep />
          <FootSends sending={1} failed={2} to="/reviews/api/89" />
        </>
      }
    />
  ),
};

export const ShortOfRoom: StoryObj = {
  render: () => (
    <div style={{ width: 300 }}>
      <Foot
        start={
          <>
            <FootSidebar shown count={0} />
            <Going />
          </>
        }
      />
    </div>
  ),
};

// The sidebar hidden: its toggle says how many need you, and brings it
// back.
function Hidden() {
  const [shown, setShown] = useState(false);
  return (
    <Foot
      start={
        <>
          <FootSidebar shown={shown} count={needs.length} onPress={() => setShown((s) => !s)} />
          <Going />
        </>
      }
    />
  );
}

export const SidebarHidden: StoryObj = { render: () => <Hidden /> };

// Beside's toggle, at the foot's end: closed with something there to peek
// at, it's marked.
function WithBeside() {
  const [open, setOpen] = useState(false);
  return (
    <Foot
      start={
        <>
          <FootSidebar shown count={0} />
          <Going />
        </>
      }
      end={<FootBeside open={open} waiting={2} onPress={() => setOpen((o) => !o)} />}
    />
  );
}

export const BesidesToggle: StoryObj = { render: () => <WithBeside /> };

// Both docks' toggles, in the order the docks are in the window: Below's,
// then Beside's at the end.
function WithDocks() {
  const [below, setBelow] = useState(true);
  const [beside, setBeside] = useState(false);
  return (
    <Foot
      start={
        <>
          <FootSidebar shown count={0} />
          <Going />
        </>
      }
      end={
        <>
          <FootBelow open={below} onPress={() => setBelow((o) => !o)} />
          <FootBeside open={beside} waiting={1} onPress={() => setBeside((o) => !o)} />
        </>
      }
    />
  );
}

export const TheDocksToggles: StoryObj = { render: () => <WithDocks /> };
