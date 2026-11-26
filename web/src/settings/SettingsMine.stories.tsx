import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../ui/Button";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsKey } from "./SettingsKey";
import { SettingsMine, SettingsMineRow, SettingsMineValue, SettingsMineWhat } from "./SettingsMine";
import { SettingsUnknown } from "./SettingsUnknown";
import { Text, textClass } from "../ui/Text";

// Set by you: the keys ws doesn't read, said first; what your file sets,
// each with what it would be otherwise and Reset (a long value is cut
// short); and what this browser keeps; then what's said where your file sets
// nothing.

const meta: Meta = { title: "Settings/SettingsMine" };
export default meta;

const mine = (section: string, what: string, key: string, value: string, otherwise: string, mono = true) => (
  <SettingsMineRow key={key}>
    <SettingsMineWhat>
      <span>
        <a href="#section" className={textClass("link")}>
          {section}
        </a>{" "}
        · {what}
      </span>
      <SettingsKey>{key}</SettingsKey>
    </SettingsMineWhat>
    <SettingsMineValue>
      <span className={mono ? textClass("mono") : undefined}>{value}</span>
      <Text dim>otherwise {otherwise}</Text>
    </SettingsMineValue>
    <Button variant="ghost" size="small">
      Reset
    </Button>
  </SettingsMineRow>
);

function Yours() {
  return (
    <>
      <SettingsUnknown keys={["ws.local.toml: crew.modle", "ws.toml: missions.reveiw_before_pr"]} />
      <SettingsBlock title={<>What ws.local.toml sets</>} count={4}>
        <SettingsMine>
          {mine("Agents", "Crew · Agent", "[crew] harness", "Claude Code", "the default, Cursor", false)}
          {mine("Agents", "Crew · Also allowed", "[crew] allowed", "claude:sonnet, claude:opus, cursor:gpt-6.2-sol, codex:gpt-6.2-codex-max", "the default, none")}
          {mine("Missions", "Review before a pull request", "[missions] review_before_pr", "Off", "the team's, On", false)}
          {mine("Appearance", "Accent", "[ui] accent", "#b8456f", "the default, ws's own")}
        </SettingsMine>
      </SettingsBlock>
      <SettingsBlock title="In this browser">
        <SettingsMine>
          <SettingsMineRow>
            <span>
              <a href="#appearance" className={textClass("link")}>
                Appearance
              </a>{" "}
              · Theme
            </span>
            <span>As the system has it</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <a href="#notifications" className={textClass("link")}>
                Notifications
              </a>{" "}
              · Desktop notifications
            </span>
            <span>Off</span>
          </SettingsMineRow>
        </SettingsMine>
      </SettingsBlock>
      <SettingsBlock title={<>What ws.local.toml sets</>} count={0}>
        <div className="quiet-line">Nothing: every setting is the team's, or ws's default.</div>
        <SettingsMine>{null}</SettingsMine>
      </SettingsBlock>
    </>
  );
}

export const SetByYou: StoryObj = {
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <Yours />
    </div>
  ),
};
