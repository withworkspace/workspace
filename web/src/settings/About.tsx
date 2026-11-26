import { Link } from "react-router";
import { getProcess, type SettingsInfo } from "../api";
import { useLoaded } from "../load";
import { shortVersion } from "../shell/version";
import { openWhatsNew } from "../shell/WhatsNew/asked";
import { Button } from "../ui/Button";
import { useWorkspace } from "../workspace/store";
import { BlockView } from "./BlockView";
import { fileName } from "./file";
import { RowOf } from "./RowOf";
import { keyOf, type Section } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsFact, SettingsFacts } from "./SettingsFacts";
import { SettingsRows } from "./SettingsRows";
import { Updates } from "./Updates";
import { Text, textClass } from "../ui/Text";

// About is which ws this is and what's out that's later, and where the
// workspace keeps what Settings shows. Its blocks are placed among those:
// Updates under the version, the daemon last.
export function About({ info, section }: { info?: SettingsInfo; section: Section }) {
  const workspace = useWorkspace();
  const process = useLoaded(getProcess, [], { keep: true }).value;
  const updates = section.blocks.find((b) => b.id === "updates");
  return (
    <>
      <SettingsBlock title="ws">
        <SettingsFacts>
          <SettingsFact term="Version">
            <Text mono title={workspace.version}>
              {workspace.version ? shortVersion(workspace.version) : "—"}
            </Text>
            <Text as="div" dim>Of the running Workspace.</Text>
          </SettingsFact>
          <SettingsFact term="What's new" id="whats-new">
            <Button size="small" onClick={openWhatsNew}>
              Show what's new
            </Button>
            <Text as="div" dim>Shown once after each upgrade.</Text>
          </SettingsFact>
        </SettingsFacts>
      </SettingsBlock>
      <SettingsBlock id="updates" title="Updates">
        <Updates running={workspace.version} />
        {updates && (
          <SettingsRows>
            {updates.rows.map((row) => (
              <RowOf key={keyOf(row)} row={row} />
            ))}
          </SettingsRows>
        )}
      </SettingsBlock>
      <SettingsBlock title="This workspace">
        <SettingsFacts>
          <SettingsFact term="Name">{workspace.name}</SettingsFact>
          {workspace.org && (
            <SettingsFact term="Organisation" mono>
              {workspace.org}
            </SettingsFact>
          )}
          {workspace.defaultBranch && (
            <SettingsFact term="Default branch" mono>
              {workspace.defaultBranch}
            </SettingsFact>
          )}
          <SettingsFact term="Location" mono>
            {workspace.root}
          </SettingsFact>
          <SettingsFact term="Team settings">
            <code title={info?.team}>{info ? fileName(info.team) : "ws.toml"}</code> <Text dim>in the config repo, committed</Text>
          </SettingsFact>
          <SettingsFact term="Your settings">
            <code title={info?.file}>{info ? fileName(info.file) : "ws.local.toml"}</code> <Text dim>local, not committed</Text>
          </SettingsFact>
          {process && (
            <SettingsFact term="Process">
              <code title={process.file}>{process.name}</code>{" "}
              <Link to="/settings/process" className={textClass("link")}>
                View
              </Link>
            </SettingsFact>
          )}
        </SettingsFacts>
      </SettingsBlock>
      {section.blocks
        .filter((b) => b.id !== "updates")
        .map((b) => (
          <BlockView key={b.id} block={b} />
        ))}
    </>
  );
}
