import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { ContextMenuProvider, useContextMenu } from "./ContextMenu";
import { Icon } from "./Icon";
import { Keys } from "./Keys";
import { MenuItem, MenuSection, MenuSeparator, MenuText } from "./Menu";
import { Row, RowLine, RowMain, Rows, RowSub, RowTitle } from "./Rows";
import { Text } from "./Text";

// A context menu: press a row with the other button of the mouse. Each
// row has a menu of its own; pressing another row while one's open moves
// straight to that row's; the last row has none, and neither have words
// that are selected, so the browser's own opens there.

const meta: Meta = { title: "Components/ContextMenu" };
export default meta;

const things = [
  { id: "billing", title: "Move billing to the new ledger", sub: "mission · in review" },
  { id: "search", title: "Search that reads what you meant", sub: "mission · working" },
  { id: "notes", title: "Notes from the offsite", sub: "page" },
];

function List() {
  const [said, say] = useState("Nothing chosen yet.");
  const menu = useContextMenu();
  return (
    <div className="page" style={{ maxWidth: 520 }}>
      <Rows>
        {things.map((t) => (
          <Row
            key={t.id}
            still
            onContextMenu={menu(t.title, () => (
              <>
                <MenuSection aria-label="What can be done to it">
                  <MenuItem id="park" textValue="Park it" onAction={() => say(`Parked ${t.id}.`)}>
                    <Icon name="pause" />
                    <MenuText>Park it</MenuText>
                  </MenuItem>
                  <MenuItem id="brief" textValue="Its brief" onAction={() => say(`The brief of ${t.id}.`)}>
                    <Icon name="flag" />
                    <MenuText>Its brief</MenuText>
                    <Keys k="b" />
                  </MenuItem>
                  <MenuItem id="archive" textValue="Archive it" tone="danger" onAction={() => say(`Archived ${t.id}.`)}>
                    <Icon name="archive" />
                    <MenuText>Archive it</MenuText>
                  </MenuItem>
                </MenuSection>
                <MenuSeparator />
                <MenuItem id="new-tab" textValue="Open in new tab" onAction={() => say(`${t.id}, in a new tab.`)}>
                  <Icon name="external" />
                  <MenuText>Open in new tab</MenuText>
                </MenuItem>
              </>
            ))}
          >
            <RowMain>
              <RowLine>
                <RowTitle>{t.title}</RowTitle>
              </RowLine>
              <RowSub>{t.sub}</RowSub>
            </RowMain>
          </Row>
        ))}
        <Row still onContextMenu={menu("Nothing", null)}>
          <RowMain>
            <RowLine>
              <RowTitle>A row with no menu of its own</RowTitle>
            </RowLine>
            <RowSub>the browser's opens</RowSub>
          </RowMain>
        </Row>
      </Rows>
      <Text as="p" dim>{said}</Text>
    </div>
  );
}

export const OnRows: StoryObj = {
  render: () => (
    <ContextMenuProvider>
      <List />
    </ContextMenuProvider>
  ),
};
