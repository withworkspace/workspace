import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { useLocation } from "react-router";
import { useMedia } from "../ui/useMedia";
import { narrowQuery, toggleBelow } from "./chromeState";
import { billing, freezes, rounding, type SampleThing } from "./dockSamples";
import { FindButton } from "./FindButton";
import { Foot } from "./Foot";
import { FootBelow } from "./FootBelow";
import { FootBeside } from "./FootBeside";
import { FootCrew } from "./FootCrew";
import { FootSep } from "./FootSep";
import { FootSidebar } from "./FootSidebar";
import { Needs } from "./Needs";
import { Places } from "./Places";
import { SampleBelow, useBelowStarted } from "./SampleBelow";
import { SampleBeside } from "./SampleBeside";
import { needs, placesFor } from "./samples";
import { Shell } from "./Shell";
import { Sidebar } from "./Sidebar";
import { useChromeStore } from "./store";
import { WorkspaceName } from "./WorkspaceName";

// The whole window, with Beside: a peek at a second thing next to the
// page. A document while you're in a conversation; a conversation while
// you're reading a document. It opens and closes from the foot's end (it
// slides, and the page's column moves with it), floats over the page or
// docks next to it from its own head, and is dragged wider by its edge.
// The sidebar hides from the foot's start, as in the app. Narrow the
// window and Beside is a sheet over the page.
//
// And with Below: what's going on, under the page, from the foot's end
// too. The page makes room for it, Beside stays next to the two of them,
// and a floating Beside ends where it begins. It's dragged taller by its
// edge, up to what the window's height lets it be. On a narrow window
// it's a sheet over the page's lower part.
//
// The page and what's beside are words standing in (dockSamples.tsx): the
// panes that draw real things come after these parts.

const meta: Meta = { title: "Chrome/Shell" };
export default meta;

const page: React.CSSProperties = { maxWidth: 720, margin: "0 auto", padding: "32px 32px 64px", overflow: "auto", height: "100%", boxSizing: "border-box" };

// A thing as the page: what's beside, as it reads with the room.
function Page({ thing }: { thing: SampleThing }) {
  return (
    <div style={page}>
      <h1 style={{ fontSize: "var(--text-xl, 22px)", margin: "0 0 4px" }}>{thing.name}</h1>
      <p className="dim" style={{ margin: "0 0 20px" }}>
        {thing.sub}
      </p>
      {thing.body}
    </div>
  );
}

// below: the window has a Below, open or closed to start with. Its state
// is the window's store's, as in the app; what's beside is kept here.
function Window({ on, beside, floats = false, closed = false, below, children }: { on: SampleThing; beside: SampleThing[]; floats?: boolean; closed?: boolean; below?: "open" | "closed"; children?: ReactNode }) {
  const { pathname } = useLocation();
  const narrow = useMedia(narrowQuery);
  const [hidden, setHidden] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [open, setOpen] = useState(!closed);
  const [over, setOver] = useState(floats);
  const [size, setSize] = useState(420);
  const place = narrow ? "drawer" : hidden ? "hidden" : "open";
  const started = useBelowStarted(below === "open");
  const low = useChromeStore((s) => s.chrome.below);
  const change = useChromeStore((s) => s.change);
  if (!started) return null;
  return (
    <div style={{ height: "100vh" }}>
      <Shell
        sidebar={place}
        drawerOpen={drawer}
        onDrawerClose={() => setDrawer(false)}
        beside={open && <SampleBeside things={beside} over={over} onOver={setOver} size={size} onSize={setSize} onClose={() => setOpen(false)} sheet={narrow} />}
        besideOver={over}
        besideSize={size}
        below={below && low.open && <SampleBelow sheet={narrow} />}
        belowSize={low.height}
        side={
          <Sidebar>
            <WorkspaceName name="Sorrel" root="/try/sorrel" />
            <FindButton />
            <Places places={placesFor(needs)} path={pathname} />
            <Needs needs={needs} path={pathname} />
          </Sidebar>
        }
        foot={
          <Foot
            start={
              <>
                <FootSidebar shown={narrow ? drawer : !hidden} count={needs.length} onPress={() => (narrow ? setDrawer((d) => !d) : setHidden((h) => !h))} />
                <FootSep />
                <FootCrew id="c1" mission="Dark mode for the dashboard" to="/missions/dark-mode-for-the-dashboard/crew/c1" />
              </>
            }
            end={
              <>
                {below && <FootBelow open={low.open} onPress={() => change(toggleBelow)} />}
                {beside.length > 0 && <FootBeside open={open} waiting={beside.length} onPress={() => setOpen((o) => !o)} />}
              </>
            }
          />
        }
      >
        <Page thing={on} />
        {children}
      </Shell>
    </div>
  );
}

// In a conversation, with the document it's about next to it.
export const ADocumentBesideAConversation: StoryObj = { render: () => <Window on={rounding} beside={[freezes]} /> };

// Reading a document, with a conversation about it next to it: the reply
// is at the pane's foot.
export const AConversationBesideADocument: StoryObj = { render: () => <Window on={billing} beside={[rounding]} /> };

// Over the page: the page keeps its width, as for a quick look.
export const Floating: StoryObj = { render: () => <Window on={billing} beside={[rounding, freezes]} floats /> };

// Closed, with something there: the foot's toggle is marked, and opens
// it.
export const ClosedWithSomethingThere: StoryObj = { render: () => <Window on={billing} beside={[rounding]} closed /> };

// Below the page: what's running and what it came to, under the page,
// which makes room.
export const BelowThePage: StoryObj = { render: () => <Window on={billing} beside={[]} below="open" /> };

// Both at once: Below under the page, Beside next to the two of them.
export const BesideAndBelow: StoryObj = { render: () => <Window on={billing} beside={[rounding]} below="open" /> };

// Beside floating with Below open: it's over the page only, and ends
// where Below begins.
export const FloatingWithBelow: StoryObj = { render: () => <Window on={billing} beside={[rounding, freezes]} floats below="open" /> };

// Below closed: its toggle in the foot opens it, and it slides up.
export const BelowClosed: StoryObj = { render: () => <Window on={billing} beside={[rounding]} below="closed" /> };
