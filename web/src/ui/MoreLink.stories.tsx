import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { MoreLink } from "./MoreLink";

// More links: one that does something here (a button), one that goes
// somewhere (a link), and one after a sentence it's part of.

const meta: Meta = { title: "Components/MoreLink" };
export default meta;

function Shows() {
  const [all, setAll] = useState(false);
  return (
    <MoreLink onClick={() => setAll(!all)} aria-expanded={all}>
      {all ? "Hide" : "Show"} 4 settled
    </MoreLink>
  );
}

const row = { display: "grid", gridTemplateColumns: "140px 1fr", gap: 12, alignItems: "baseline" } as const;

export const Links: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <div style={row}>
        <span className="dim">Shown here</span>
        <span>
          <Shows />
        </span>
      </div>
      <div style={row}>
        <span className="dim">Somewhere else</span>
        <span>
          <MoreLink to="/missions/export-csv/crew/api-1">Open api-1's conversation</MoreLink>
        </span>
      </div>
      <div style={row}>
        <span className="dim">In a sentence</span>
        <span className="dim">
          Two capsules, one with a pull request open. <MoreLink onClick={() => {}}>All of it in Work</MoreLink>
        </span>
      </div>
    </div>
  ),
};
