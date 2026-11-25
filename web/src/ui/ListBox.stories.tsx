import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Chip } from "./Chip";
import { ListBox, ListBoxOption } from "./ListBox";
import { Text } from "./Text";

// A list to pick one of: none picked, one picked, one that can't be, and
// empty. Press a row, or move with the arrows and press Enter.

const meta: Meta = { title: "Components/ListBox" };
export default meta;

const repos = [
  { name: "api", about: "The invoices API", here: false },
  { name: "web", about: "The dashboard", here: false },
  { name: "records", about: "ADRs, proposals and notes", here: true },
  { name: "billing-legacy", about: "Archived: the old billing service", here: false },
];

const box = { width: 380, border: "1px solid var(--line)", borderRadius: 8, overflow: "hidden" } as const;
const row = { display: "flex", flexDirection: "column", gap: 2, padding: "8px 12px", borderBottom: "1px solid var(--line)" } as const;

function Repos({ start, none }: { start?: string; none?: boolean }) {
  const [chosen, setChosen] = useState(start);
  return (
    <div style={box}>
      <ListBox value={chosen} onChange={setChosen} aria-label="Sorrel's repos" empty={<Text as="p" dim style={{ margin: 0, padding: 16 }}>Nothing to add here.</Text>}>
        {(none ? [] : repos).map((r) => (
          <ListBoxOption key={r.name} value={r.name} textValue={r.name} disabled={r.here} title={r.here ? "Here already" : undefined}>
            <span style={row}>
              <span style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Text mono>{r.name}</Text>
                {r.here && <Chip tone="done">here</Chip>}
              </span>
              <Text dim>{r.about}</Text>
            </span>
          </ListBoxOption>
        ))}
      </ListBox>
    </div>
  );
}

export const Lists: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 24, alignItems: "start", flexWrap: "wrap" }}>
      <Repos />
      <Repos start="web" />
      <Repos none />
    </div>
  ),
};
