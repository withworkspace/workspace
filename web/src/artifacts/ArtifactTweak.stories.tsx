import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { Icon } from "../ui/Icon";
import { ArtifactTweak, ArtifactTweakList } from "./ArtifactTweak";
import { ArtifactTweakChoice } from "./ArtifactTweakChoice";
import { ArtifactTweakColour } from "./ArtifactTweakColour";
import { ArtifactTweakNumber } from "./ArtifactTweakNumber";
import { ArtifactTweakRange } from "./ArtifactTweakRange";
import { ArtifactTweakSelect } from "./ArtifactTweakSelect";
import { ArtifactTweakSwitch } from "./ArtifactTweakSwitch";
import { ArtifactTweakText } from "./ArtifactTweakText";
import { ArtifactTweaksButton } from "./ArtifactTweaksButton";
import { Page, problems, suggested, told, useTold } from "./samples";
import type { State, Told } from "./tweaks";
import { TweaksBody } from "./TweaksBody";
import { TweaksHead } from "./TweaksHead";
import { TweaksToggle } from "./TweaksToggle";
import { TweaksTray } from "./TweaksTray";
import { textClass } from "../ui/Text";

// An artifact's tweaks, from their parts: each control, as it opens and
// changed (which is marked, and says how it opens); the tray beside the
// frame, as the page opens, with some changed (how many, and the way to
// compare it with how it opens), and with what's wrong with how the page
// declares them; and the same behind a button, a comparison's side.

const meta: Meta = { title: "Artifacts/Tweaks" };
export default meta;

function Column({ children }: { children: ReactNode }) {
  return <div style={{ width: 268, padding: 16 }}>{children}</div>;
}

function Controls({ changed }: { changed?: boolean }) {
  const [layout, setLayout] = useState(changed ? "list" : "grid");
  const [theme, setTheme] = useState(changed ? "Always dark" : "Follows the system");
  const [shadows, setShadows] = useState(!changed);
  const [radius, setRadius] = useState(changed ? "14" : "8");
  const [columns, setColumns] = useState(changed ? "four" : "3");
  const [accent, setAccent] = useState(changed ? "#2f7d5b" : "#3a6ea5");
  const [heading, setHeading] = useState(changed ? "Every control, changed" : "Every control");
  const opens = (said: string) => (changed ? said : undefined);
  return (
    <Column>
      <ArtifactTweakList>
        <ArtifactTweak id="t-layout" kind="choice" label="Layout" description="How the cards sit." opensAs={opens("Grid")} chosen={layout === "list" ? "One to a row, its note beside it." : ""}>
          <ArtifactTweakChoice
            id="t-layout"
            label="Layout"
            value={layout}
            onChange={setLayout}
            options={[
              { value: "grid", label: "Grid", description: "" },
              { value: "list", label: "List", description: "One to a row, its note beside it." },
            ]}
          />
        </ArtifactTweak>
        <ArtifactTweak id="t-theme" kind="choice" label="Theme" description="A longer list of choices is a dropdown." opensAs={opens("Follows the system")}>
          <ArtifactTweakSelect id="t-theme" label="Theme" value={theme} onChange={setTheme} options={["Follows the system", "Always light", "Always dark", "High contrast, light", "High contrast, dark"].map((value) => ({ value, label: value, description: "" }))} />
        </ArtifactTweak>
        <ArtifactTweak id="t-shadows" kind="toggle" label="Shadows" description="A soft shadow under each card." opensAs={opens("on")} inline>
          <ArtifactTweakSwitch id="t-shadows" label="Shadows" on={shadows} onChange={setShadows} />
        </ArtifactTweak>
        <ArtifactTweak id="t-radius" kind="number" label="Corner radius" opensAs={opens("8px")}>
          <ArtifactTweakRange id="t-radius" min={0} max={24} step={1} value={radius} said={`${radius}px`} onChange={setRadius} />
        </ArtifactTweak>
        <ArtifactTweak id="t-columns" kind="number" label="Columns" description="A number with no ends is typed." opensAs={opens("3cols")}>
          <ArtifactTweakNumber id="t-columns" value={columns} unit="cols" invalid={!/^\d+$/.test(columns)} onChange={setColumns} />
        </ArtifactTweak>
        <ArtifactTweak id="t-accent" kind="color" label="Accent" opensAs={opens("#3a6ea5")} inline>
          <ArtifactTweakColour id="t-accent" label="Accent" value={accent} onChange={setAccent} />
        </ArtifactTweak>
        <ArtifactTweak id="t-heading" kind="text" label="Heading" description="What the page is called." opensAs={opens("Every control")}>
          <ArtifactTweakText id="t-heading" value={heading} onChange={setHeading} />
        </ArtifactTweak>
      </ArtifactTweakList>
    </Column>
  );
}

export const EachControl: StoryObj = {
  render: () => (
    <div style={{ display: "flex", alignItems: "start", gap: 24 }}>
      <Controls />
      <Controls changed />
    </div>
  ),
};

function Tray({ first, compare }: { first: Told; compare?: boolean }) {
  const [said, set] = useTold(first);
  const [shown, setShown] = useState(true);
  return (
    <Page
      title="Plan list"
      tweaks={<TweaksToggle told={said} shown={shown} onToggle={() => setShown((x) => !x)} />}
      tray={
        shown && (
          <TweaksTray told={said} set={set} onClose={() => setShown(false)}>
            {compare && (
              <a className={textClass("link")} href="#compare" title="The same version twice, side by side: as it opens, and as it's set here">
                <Icon name="split" size={12} /> Compare with how it opens
              </a>
            )}
          </TweaksTray>
        )
      }
    />
  );
}

const set = (state: State): Told => ({ ...told, state: { ...told.state, ...state } });

export const TrayAsItOpens: StoryObj = {
  render: () => <Tray first={told} />,
};

export const TrayChanged: StoryObj = {
  render: () => <Tray first={set(suggested)} compare />,
};

export const TrayWithProblems: StoryObj = {
  render: () => <Tray first={{ ...told, problems }} />,
};

// A page that declares one tweak and no presets: all there is to change.
export const TrayWithLittle: StoryObj = {
  render: () => <Tray first={{ tweaks: [told.tweaks[2]], presets: [], opening: { pips: false }, state: { pips: false }, problems: [] }} />,
};

function Behind({ first }: { first: Told }) {
  const [said, setSaid] = useTold(first);
  return (
    <div style={{ display: "flex", justifyContent: "end", padding: 16, height: 640 }}>
      <ArtifactTweaksButton label="The right one's tweaks" changed={Object.keys(said.state).filter((k) => said.state[k] !== said.opening[k]).length} defaultOpen>
        <TweaksHead told={said} />
        <TweaksBody told={said} set={setSaid} />
      </ArtifactTweaksButton>
    </div>
  );
}

export const BehindAButton: StoryObj = {
  render: () => <Behind first={set(suggested)} />,
};
