import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { useLocation } from "react-router";
import { useMedia } from "../ui/kit";
import { narrowQuery } from "./chromeState";
import { billing, freezes, rounding, type SampleThing } from "./dockSamples";
import { FindButton } from "./FindButton";
import { Foot } from "./Foot";
import { FootBeside } from "./FootBeside";
import { FootCrew } from "./FootCrew";
import { FootSep } from "./FootSep";
import { FootSidebar } from "./FootSidebar";
import { Needs } from "./Needs";
import { Places } from "./Places";
import { SampleBeside } from "./SampleBeside";
import { needs, placesFor } from "./samples";
import { Shell } from "./Shell";
import { Sidebar } from "./Sidebar";
import { WorkspaceName } from "./WorkspaceName";

// The whole window, with Beside: a peek at a second thing next to the
// page. A document while you're in a conversation; a conversation while
// you're reading a document. It opens and closes from the foot's end (it
// slides, and the page's column moves with it), floats over the page or
// docks next to it from its own head, and is dragged wider by its edge.
// The sidebar hides from the foot's start, as in the app. Narrow the
// window and Beside is a sheet over the page.
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

function Window({ on, beside, floats = false, closed = false, children }: { on: SampleThing; beside: SampleThing[]; floats?: boolean; closed?: boolean; children?: ReactNode }) {
  const { pathname } = useLocation();
  const narrow = useMedia(narrowQuery);
  const [hidden, setHidden] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [open, setOpen] = useState(!closed);
  const [over, setOver] = useState(floats);
  const [size, setSize] = useState(420);
  const place = narrow ? "drawer" : hidden ? "hidden" : "open";
  return (
    <div style={{ height: "100vh" }}>
      <Shell
        sidebar={place}
        drawerOpen={drawer}
        onDrawerClose={() => setDrawer(false)}
        beside={open && <SampleBeside things={beside} over={over} onOver={setOver} size={size} onSize={setSize} onClose={() => setOpen(false)} sheet={narrow} />}
        besideOver={over}
        besideSize={size}
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
            end={<FootBeside open={open} waiting={beside.length} onPress={() => setOpen((o) => !o)} />}
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
