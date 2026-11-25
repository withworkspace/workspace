import { Roles } from "./Roles";
import { RowOf } from "./RowOf";
import { type Block, keyOf } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsRows } from "./SettingsRows";

// BlockView is a block: its heading, what it says, and its rows (the
// roles' table, for theirs). Its id is what an address's hash names.
export function BlockView({ block }: { block: Block }) {
  if (block.roles) {
    if (block.roles.length === 0) return null;
    return (
      <SettingsBlock id={block.id}>
        <Roles roles={block.roles} />
      </SettingsBlock>
    );
  }
  if (block.rows.length === 0) return null;
  return (
    <SettingsBlock id={block.id} title={block.title} about={block.about}>
      <SettingsRows>
        {block.rows.map((row) => (
          <RowOf key={keyOf(row)} row={row} />
        ))}
      </SettingsRows>
    </SettingsBlock>
  );
}
