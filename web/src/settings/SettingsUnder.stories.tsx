import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Setting } from "../api";
import { Choice } from "./Choice";
import { yours } from "./samples";
import { fromPlugins } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsRow } from "./SettingsRow";
import { SettingsRows } from "./SettingsRows";
import { SettingsUnder } from "./SettingsUnder";

// Notifications' blocks from plugins: under Plugins, a block for each
// plugin, a row for each kind it tells of, sent where you choose.

const meta: Meta = { title: "Settings/SettingsUnder" };
export default meta;

const routes = [
  { value: "feed", label: "Feed" },
  { value: "desktop", label: "Feed, and tell me" },
  { value: "off", label: "Off" },
];

// Each kind: its id, title, about, where it's sent, and where the plugin
// said by default.
const kinds: Record<string, [string, string, string, string, string][]> = {
  jira: [
    ["assigned", "Given to you", "Someone else gives you an issue on the board.", "desktop", "feed"],
    ["comment", "Comments on yours", "Someone comments on an issue that's yours.", "feed", "feed"],
    ["moved", "Yours moved", "Someone moves an issue that's yours to another status.", "feed", "feed"],
    ["new", "New on the board", "An issue a query finds that the board hadn't.", "off", "off"],
  ],
  sketch: [["shared", "Shared with you", "Someone puts a sketch in a repo you're in.", "feed", "feed"]],
};

export const FromPlugins: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <SettingsBlock title="What happened" about="Where each of ws's own sources is sent." />
      <SettingsUnder title={fromPlugins.title} about={fromPlugins.about} />
      {Object.entries(kinds).map(([plugin, rows]) => (
        <SettingsBlock key={plugin} title={plugin} about={`What ${plugin} tells you of, kind by kind: where each is sent.`}>
          <SettingsRows>
            {rows.map(([key, label, about, value, def]) => {
              const s = { section: `notifications.plugin.${plugin}`, key, label, about, kind: "choice", value, default: def, source: value === def ? "default" : "yours", choices: routes } as Setting;
              return (
                <SettingsRow key={key} label={label} tomlKey={`[${s.section}] ${key}`} about={about} source={value === def ? undefined : yours(`default ${def}`)}>
                  <Choice s={s} set={async () => {}} reset={() => {}} />
                </SettingsRow>
              );
            })}
          </SettingsRows>
        </SettingsBlock>
      ))}
    </div>
  ),
};
