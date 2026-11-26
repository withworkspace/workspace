import { Link } from "react-router";
import { PageHead } from "../ui/PageHead";
import { RowOf } from "./RowOf";
import { type Found, keyOf } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsRows } from "./SettingsRows";
import { Text } from "../ui/Text";

// Results is what Find found: each section's rows that say it, working
// as they do in their section.
export function Results({ found, query, clear }: { found: Found[]; query: string; clear: () => void }) {
  const n = found.reduce((sum, f) => sum + f.rows.length, 0);
  return (
    <>
      <PageHead title={<>“{query.trim()}”</>}>
        <Text dim>
          {n} {n === 1 ? "setting" : "settings"}
        </Text>
      </PageHead>
      {found.length === 0 && <div className="quiet-line">No matching setting. File-only settings (repo commands, paths) aren't listed.</div>}
      {found.map(({ section, rows }) => (
        <SettingsBlock
          key={section.id}
          title={
            <Link to={`/settings/${section.id}`} onClick={clear}>
              {section.title}
            </Link>
          }
        >
          <SettingsRows>
            {rows.map((row) => (
              <RowOf key={keyOf(row)} row={row} found />
            ))}
          </SettingsRows>
        </SettingsBlock>
      ))}
    </>
  );
}
