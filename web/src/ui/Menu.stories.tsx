import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Count } from "./Count";
import { Icon } from "./Icon";
import { Keys } from "./kit";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuText } from "./Menu";

// Menus: press each to open it. Things to do, with icons, a count, a key
// and a danger one; sections with titles; and a foot that isn't an item.

const meta: Meta = { title: "Components/Menu" };
export default meta;

function Said({ children }: { children: (say: (what: string) => void) => React.ReactNode }) {
  const [said, say] = useState("Nothing chosen yet.");
  return (
    <div className="page" style={{ display: "flex", gap: 16, alignItems: "center" }}>
      {children(say)}
      <span className="dim">{said}</span>
    </div>
  );
}

export const ThingsToDo: StoryObj = {
  render: () => (
    <Said>
      {(say) => (
        <Menu aria-label="The workspace" trigger={<Button>Sorrel <Icon name="chevron-down" size={12} /></Button>} onAction={(id) => say(`Chose ${id}.`)}>
          <MenuSection aria-label="Places">
            <MenuItem id="reviews" textValue="Reviews">
              <Icon name="pr" />
              <MenuText>Reviews</MenuText>
              <Count tone="attention">2</Count>
            </MenuItem>
            <MenuItem id="team" textValue="Team">
              <Icon name="user" />
              <MenuText>Team</MenuText>
            </MenuItem>
          </MenuSection>
          <MenuSeparator />
          <MenuItem id="settings" textValue="Settings">
            <Icon name="sliders" />
            <MenuText>Settings</MenuText>
          </MenuItem>
          <MenuItem id="keys" textValue="Keys">
            <Icon name="keyboard" />
            <MenuText>Keys</MenuText>
            <Keys k="?" />
          </MenuItem>
          <MenuSeparator />
          <MenuItem id="shutdown" textValue="Shut down the daemon" tone="danger">
            <Icon name="power" />
            <MenuText>Shut down the daemon…</MenuText>
          </MenuItem>
        </Menu>
      )}
    </Said>
  ),
};

export const SectionsWithTitles: StoryObj = {
  render: () => (
    <Said>
      {(say) => (
        <Menu
          aria-label="Bundles"
          placement="bottom"
          onAction={(id) => say(`Opened ${id}.`)}
          trigger={
            <Button variant="ghost">
              3 other bundles <Icon name="chevron-down" size={12} />
            </Button>
          }
        >
          <MenuSection title="Yours to verify">
            <MenuItem id="b2" textValue="b2 Round once, at the end">
              <span className="ref">b2</span>
              <MenuText>Round once, at the end</MenuText>
            </MenuItem>
          </MenuSection>
          <MenuSeparator />
          <MenuSection title="Approved">
            <MenuItem id="b1" textValue="b1 Read the totals">
              <span className="ref">b1</span>
              <MenuText>Read the totals, which is a name long enough that it's cut short where the menu ends</MenuText>
            </MenuItem>
            <MenuItem id="b0" textValue="b0 A penny doesn't go missing">
              <span className="ref">b0</span>
              <MenuText>A penny doesn't go missing</MenuText>
            </MenuItem>
          </MenuSection>
        </Menu>
      )}
    </Said>
  ),
};

export const WithAFoot: StoryObj = {
  render: () => (
    <Said>
      {(say) => (
        <Menu
          aria-label="Panes"
          onAction={(id) => say(`${id} panes.`)}
          trigger={
            <Button variant="ghost" square aria-label="Panes">
              <Icon name="panel" size={14} />
            </Button>
          }
          foot={
            <div className="dim" style={{ padding: "6px 10px 8px", borderTop: "1px solid var(--line)", fontSize: "var(--text-xs)" }}>
              Switch with <Keys k="p" />
            </div>
          }
        >
          {["3", "2", "1"].map((n) => (
            <MenuItem key={n} id={n} textValue={`${n} panes`}>
              <MenuText>{n === "1" ? "The file alone" : `${n} panes`}</MenuText>
              {n === "2" && <Icon name="check" size={13} />}
            </MenuItem>
          ))}
        </Menu>
      )}
    </Said>
  ),
};
