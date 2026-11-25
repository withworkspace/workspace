import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Sidebar } from "../../chrome/Sidebar";
import { workspace } from "../../chrome/samples";
import { arrange, move, placesOf } from "../places";
import { Arrange } from "./Arrange";

// The sidebar while it's arranged, side by side: Sorrel's places as they
// come (Runs in the menu); with nothing in the menu (what's said in an
// empty list); with more moved into the menu; and with nothing left in
// the sidebar. Each is
// the real thing: drag a place by its grip to see it lifted and the line
// where it'd go (a drag can't be held still in a story), or press its
// arrow to move it across.

const meta: Meta = { title: "Shell/Arrange" };
export default meta;

const places = arrange(placesOf(workspace, []));
const emptyMenu = move(places, places.menu.map((p) => p.id), "sidebar");
const more = move(places, ["team", "files", "orchestrator"], "menu");
const none = move(places, places.sidebar.map((p) => p.id), "menu");

function ArrangeIn({ start, canReset }: { start: typeof places; canReset?: boolean }) {
  const [said, say] = useState<string>();
  return (
    <div style={{ display: "grid", gap: 8, alignContent: "start" }}>
      <Sidebar>
        <Arrange
          start={start}
          canReset={!!canReset}
          onDone={(a) => say(`Done: ${a.sidebar.map((p) => p.label).join(", ") || "nothing"}; in the menu: ${a.menu.map((p) => p.label).join(", ") || "nothing"}`)}
          onCancel={() => say("Cancelled")}
          onReset={() => say("Reset")}
        />
      </Sidebar>
      {said && (
        <p className="dim" style={{ maxWidth: 232, margin: 0 }}>
          {said}
        </p>
      )}
    </div>
  );
}

export const Arranging: StoryObj = {
  render: () => (
    <div style={{ display: "flex", gap: 24, alignItems: "start", padding: 12, background: "var(--bg)", minHeight: 640 }}>
      <ArrangeIn start={places} />
      <ArrangeIn start={emptyMenu} canReset />
      <ArrangeIn start={more} canReset />
      <ArrangeIn start={none} canReset />
    </div>
  ),
};
