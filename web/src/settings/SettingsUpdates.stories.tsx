import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../ui/Button";
import { none, updates } from "./samples";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsFact, SettingsFacts } from "./SettingsFacts";
import { SettingsUpdates } from "./SettingsUpdates";
import { type Standing, standingOf } from "./standing";

// Settings' About: the facts of which ws this is and where the workspace
// keeps its files, and Updates in each standing: this is the latest; this
// ws is later; a later one's out, to upgrade to here, and while it's
// being installed; one's out but this ws was built from a clone; one's
// installed and the daemon's waiting to restart onto it (for something,
// and for nothing); a look that got no answer; never looked, and the
// daemon doesn't; and plugins with later releases.

const meta: Meta = { title: "Settings/SettingsUpdates" };
export default meta;

const out = { latest: "v0.16.0", newer: true, page: "https://github.com/withworkspace/workspace/releases/tag/v0.16.0" };

const each: [string, ReturnType<typeof updates>, ("look" | "upgrade")?][] = [
  ["This is the latest", updates()],
  ["This ws is later", updates({ version: "v0.16.0-rc.1" })],
  ["A later one is out", updates(out)],
  ["Being installed", updates(out), "upgrade"],
  ["Out, and this ws was built from a clone", updates({ ...out, release: false, version: "v0.15.1-0.20261125083702-633485334942" })],
  ["Installed, waiting for crew", updates({ ...out, installed: "v0.16.0", waits: ["c2, mid-turn on Limit each key", "the orchestrator's run"] })],
  ["Installed, waiting for nothing", updates({ ...out, installed: "v0.16.0" })],
  ["A look that got no answer", updates({ error: "Get \"https://api.github.com/repos/withworkspace/workspace/releases/latest\": dial tcp: lookup api.github.com: no such host", tried: "2026-11-20T15:10:00Z" })],
  ["Never looked, and the daemon doesn't", updates({ checks: false, checked: undefined, latest: undefined, page: undefined })],
  ["Looking now, with plugins later", updates({ plugins: 2 }), "look"],
];

export const Updates: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      {each.map(([title, up, busy]) => (
        <SettingsBlock key={title} title={title}>
          <SettingsUpdates up={up} standing={standingOf(up) as Standing} busy={busy} onLook={none} onUpgrade={none} />
        </SettingsBlock>
      ))}
    </div>
  ),
};

export const Facts: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <SettingsBlock title="ws">
        <SettingsFacts>
          <SettingsFact term="Version">
            <span className="mono">v0.15.0</span>
            <div className="dim">The daemon's, which serves this page.</div>
          </SettingsFact>
          <SettingsFact term="What's new" id="whats-new">
            <Button size="small">Show what's new</Button>
            <div className="dim">Said once after an upgrade, and kept here.</div>
          </SettingsFact>
        </SettingsFacts>
      </SettingsBlock>
      <SettingsBlock title="This workspace">
        <SettingsFacts>
          <SettingsFact term="Name">Sorrel</SettingsFact>
          <SettingsFact term="Organisation" mono>
            sorrel
          </SettingsFact>
          <SettingsFact term="Where it is" mono>
            /Users/you/Projects/sorrel/a-folder-with-a-long-name-that-wraps-where-it-has-to/workspace
          </SettingsFact>
          <SettingsFact term="The team's settings">
            <code>ws.toml</code> <span className="dim">in the config repo, changed by commit</span>
          </SettingsFact>
          <SettingsFact term="Yours">
            <code>ws.local.toml</code> <span className="dim">beside it, not committed</span>
          </SettingsFact>
        </SettingsFacts>
      </SettingsBlock>
    </div>
  ),
};
