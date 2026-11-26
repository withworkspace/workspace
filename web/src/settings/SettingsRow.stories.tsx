import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { Switch } from "../ui/Switch";
import { Agent, browser, Field, Model, none, Number, PickList, team, yours } from "./samples";
import { SettingsAccent } from "./SettingsAccent";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsMarked } from "./SettingsMarked";
import { SettingsNote } from "./SettingsNote";
import { SettingsRow } from "./SettingsRow";
import { SettingsTint } from "./SettingsTint";
import { SettingsRows } from "./SettingsRows";

// The Settings page's rows: one of each control (a switch, a few to pick
// from, a list, what's typed, a number, picks, the accent), each source a
// value can have (ws's default says nothing; the team's; yours, over the
// team's or the default; this browser), and a row out of its block with
// what Find typed marked in it. In a window 640px wide or less a row's
// control goes under its words.

const meta: Meta = { title: "Settings/SettingsRow" };
export default meta;

const accents: [string, string][] = [
  ["#4a5bd0", "Indigo"],
  ["#2b6cb8", "Blue"],
  ["#0e7f86", "Teal"],
  ["#2f7d4f", "Green"],
  ["#a8650f", "Amber"],
  ["#b8456f", "Rose"],
  ["#59626f", "Slate"],
];

function Toggle({ on: was }: { on: boolean }) {
  const [on, setOn] = useState(was);
  return <Switch on={on} onChange={setOn} aria-label="A switch" />;
}

function Where() {
  const [v, setV] = useState("web");
  return (
    <Segmented value={v} onChange={setV} aria-label="Where">
      <Segment value="web">The web UI</Segment>
      <Segment value="terminal">A terminal</Segment>
    </Segmented>
  );
}

function Accent({ picked: was, pulled }: { picked: string; pulled?: { way: string; as: string } }) {
  const [picked, setPicked] = useState(was);
  const [hex, setHex] = useState<string | null>(null);
  return (
    <SettingsAccent
      label="Accent"
      ours="#5b5bd6"
      accents={accents}
      picked={picked}
      hex={hex ?? picked}
      dark={document.documentElement.getAttribute("data-theme") === "dark"}
      pulled={pulled}
      onOurs={() => setPicked("#5b5bd6")}
      onPick={setPicked}
      onPreview={setPicked}
      onHexFocus={() => setHex(picked)}
      onHexChange={setHex}
      onHexBlur={(text) => {
        if (/^#[0-9a-f]{6}$/i.test(text.trim())) setPicked(text.trim());
        setHex(null);
      }}
      onHexCancel={() => setHex(null)}
    />
  );
}

function Tint({ value: was }: { value: number }) {
  const [value, setValue] = useState(was);
  return <SettingsTint label="Tint" value={value} onPreview={setValue} />;
}

function Rows() {
  return (
    <>
      <SettingsBlock title="Each control" about="A change is written at once: there's nothing to save.">
        <SettingsRows>
          <SettingsRow label="Review before a pull request" tomlKey="[missions] review_before_pr" about="A mission's change is yours to read before ws opens its pull request." source={team}>
            <Toggle on />
          </SettingsRow>
          <SettingsRow label="Where a brief is written" tomlKey="[briefing] in" about="The planner asks in the web UI, or in a terminal of its own.">
            <Where />
          </SettingsRow>
          <SettingsRow label="Agent" tomlKey="[crew] harness" about="The agent CLI crew run in." source={yours("default Cursor")}>
            <Agent />
          </SettingsRow>
          <SettingsRow label="Model" tomlKey="[crew] model" about="A list the agent gave, with another by name.">
            <Model />
          </SettingsRow>
          <SettingsRow label="How far back the Team page looks" tomlKey="[team] window" about="A length of time: 14d, 2w." source={yours("default 14d")}>
            <Field value="7d" short />
          </SettingsRow>
          <SettingsRow label="Rounds of review" tomlKey="[missions] max_rounds" about="How many times a change goes back before it's yours to decide." source={yours("the team's is 3")}>
            <Number value={5} />
          </SettingsRow>
          <SettingsRow label="Also allowed" tomlKey="[crew] allowed" about="Agents and models a plan may name for a step, beside the crew's own." wide source={yours()}>
            <PickList picks={["claude:sonnet", "claude:opus", "cursor:gpt-6.2-sol"]} />
          </SettingsRow>
          <SettingsRow label="Also allowed, with none" tomlKey="[crew] allowed" wide>
            <PickList picks={[]} />
          </SettingsRow>
        </SettingsRows>
      </SettingsBlock>
      <SettingsBlock title="The accent">
        <SettingsRows>
          <SettingsRow label="Accent" tomlKey="[ui] accent" about="ws's own, one of a few, or any other: the page wears it as you pick.">
            <Accent picked="#5b5bd6" />
          </SettingsRow>
          <SettingsRow label="Accent, one of those to pick from" tomlKey="[ui] accent" source={yours()}>
            <Accent picked="#b8456f" />
          </SettingsRow>
          <SettingsRow label="Accent, another, which the theme darkens" tomlKey="[ui] accent" source={yours()}>
            <Accent picked="#ffd84d" pulled={{ way: "darker", as: "#8a6d00" }} />
          </SettingsRow>
          <SettingsRow label="Tint" tomlKey="[ui] tint" about="How far the accent tints the page, the panels, lines and text." source={yours()}>
            <Tint value={40} />
          </SettingsRow>
        </SettingsRows>
      </SettingsBlock>
      <SettingsBlock title="The page's own">
        <SettingsRows>
          <SettingsRow label="Desktop notifications" about="When something needs you and this tab isn't in front. This browser has none." source={browser}>
            <Switch on={false} disabled onChange={none} aria-label="Desktop notifications" />
          </SettingsRow>
          <SettingsRow label="Sidebar" tomlKey="[ui] sidebar · menu" about="Which places are in the sidebar, and in what order." source={yours("", true)}>
            <Button type="button">
              <Icon name="grip" size={13} />
              Arrange the sidebar…
            </Button>
          </SettingsRow>
          <SettingsRow label="The daemon" about="Shut it down: every crew's session ends with it.">
            <Button variant="danger" type="button">
              <Icon name="power" size={13} />
              Shut down…
            </Button>
          </SettingsRow>
        </SettingsRows>
      </SettingsBlock>
      <SettingsBlock title="Found, out of its block">
        <SettingsRows>
          <SettingsRow label={<SettingsMarked text="Model" words={["model"]} />} tomlKey={<SettingsMarked text="[judge] model" words={["model"]} />} crumb="Judge" about={<SettingsMarked text="The model the judge is given, in its agent's own naming." words={["model"]} />} source={yours("default sonnet")}>
            <Field value="haiku" mono />
            <SettingsNote>Couldn't list codex's models: codex isn't on this machine</SettingsNote>
          </SettingsRow>
        </SettingsRows>
      </SettingsBlock>
    </>
  );
}

export const Rows_: StoryObj = {
  name: "Rows",
  render: () => (
    <div style={{ maxWidth: 824, margin: "0 auto", padding: 32 }}>
      <Rows />
    </div>
  ),
};
