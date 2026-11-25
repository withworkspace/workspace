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
            <span className="mono" title={workspace.version}>
              {workspace.version ? shortVersion(workspace.version) : "—"}
            </span>
            <div className="dim">The daemon's, which serves this page.</div>
          </SettingsFact>
          <SettingsFact term="What's new" id="whats-new">
            <Button size="small" onClick={openWhatsNew}>
              Show what's new
            </Button>
            <div className="dim">Said once after an upgrade, and kept here.</div>
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
          <SettingsFact term="Where it is" mono>
            {workspace.root}
          </SettingsFact>
          <SettingsFact term="The team's settings">
            <code title={info?.team}>{info ? fileName(info.team) : "ws.toml"}</code> <span className="dim">in the config repo, changed by commit</span>
          </SettingsFact>
          <SettingsFact term="Yours">
            <code title={info?.file}>{info ? fileName(info.file) : "ws.local.toml"}</code> <span className="dim">beside it, not committed</span>
          </SettingsFact>
          {process && (
            <SettingsFact term="Process">
              <code title={process.file}>{process.name}</code>{" "}
              <Link to="/settings/process" className="link">
                as it's written
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
