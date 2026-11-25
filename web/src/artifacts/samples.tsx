import { type ReactNode, useState } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { ArtifactActions } from "./ArtifactActions";
import { ArtifactBody, ArtifactStage } from "./ArtifactBody";
import { ArtifactFrame } from "./ArtifactFrame";
import { ArtifactHead } from "./ArtifactHead";
import { ArtifactNote } from "./ArtifactNote";
import { ArtifactTitleLine, ArtifactTitleRef } from "./ArtifactTitleLine";
import { ArtifactTools } from "./ArtifactTools";
import { ArtifactVersionLine } from "./ArtifactVersionLine";
import { ArtifactVersions, type ArtifactVersionsItem } from "./ArtifactVersions";
import { ArtifactView } from "./ArtifactView";
import { artifactStatus, widths } from "./shown";
import type { State, Told } from "./tweaks";

// What the Artifacts stories draw: times about now, a page for a frame to
// run, an artifact's versions, what a page declares of its tweaks (kept as
// it's set, as a frame would answer), and an artifact's page from its
// parts.

export const none = () => {};

export const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();
export const days = (n: number) => ago(n * 24 * 60);

// page is a version to run in a story's frame: a page that says what it
// is, as a data: address.
export const page = (says: string, ground = "#f6f5f2") =>
  `data:text/html,${encodeURIComponent(`<!doctype html><body style="margin:0;display:grid;place-items:center;min-height:100vh;background:${ground};font:15px/1.5 system-ui,sans-serif;color:#333"><p>${says}</p></body>`)}`;

export const live = artifactStatus({ status: "live", gone: false });
export const promoted = artifactStatus({ status: "promoted", gone: false });
export const discarded = artifactStatus({ status: "discarded", gone: false });
export const expired = artifactStatus({ status: "live", gone: false, expired: true });
export const gone = artifactStatus({ status: "live", gone: true });

export const versions: ArtifactVersionsItem[] = [
  { n: 1, by: "by you", at: days(3), note: "Export at the end of the toolbar, after the filter: it's the last thing you do with a list.", home: { repo: "records", where: "records:artifacts/2026-11-20-where-export-goes/" } },
  { n: 2, by: "by c1", at: days(1), note: "Export under the list, beside the count it exports, so the button says how many." },
  { n: 3, by: "by you", at: ago(40) },
];

// What a page declares: one tweak of each kind, and three presets.
export const told: Told = {
  tweaks: [
    { name: "density", label: "Density", description: "How tall a step's row is.", kind: "choice", options: [{ value: "cozy", label: "Cozy", description: "" }, { value: "compact", label: "Compact", description: "28px rows, the step's note dropped." }] },
    {
      name: "shown",
      label: "Steps shown",
      description: "How much of a long plan is on the page.",
      kind: "choice",
      options: ["All of them", "2 done + 3 next", "The current step only", "None, folded"].map((value) => ({ value, label: value, description: "" })),
    },
    { name: "pips", label: "Progress pips", description: "A dot for each step, under the heading.", kind: "toggle" },
    { name: "radius", label: "Corner radius", description: "", kind: "number", min: 0, max: 16, unit: "px" },
    { name: "columns", label: "Columns", description: "A number with no ends is typed.", kind: "number", unit: "cols" },
    { name: "accent", label: "Accent", description: "", kind: "color" },
    { name: "heading", label: "Heading", description: "", kind: "text" },
  ],
  presets: [
    { label: "As it is today", description: "What a mission's page shows now.", state: { density: "cozy", shown: "All of them", pips: false, radius: 6, columns: 3, accent: "#3a6ea5", heading: "Plan" } },
    { label: "Suggested", description: "A window on a long plan, with pips to say where it's at.", state: { density: "compact", shown: "2 done + 3 next", pips: true } },
    { label: "Quietest", description: "", state: { shown: "The current step only", pips: false } },
  ],
  opening: { density: "cozy", shown: "All of them", pips: false, radius: 6, columns: 3, accent: "#3a6ea5", heading: "Plan" },
  state: { density: "cozy", shown: "All of them", pips: false, radius: 6, columns: 3, accent: "#3a6ea5", heading: "Plan" },
  problems: [],
};

export const suggested: State = { density: "compact", shown: "2 done + 3 next", pips: true };

export const problems = ["9lives: a tweak's name is letters, digits, - and _, starting with a letter", "empty: it has no options", "Flat: there's no tweak named nothere"];

// useTold is what a frame's page declared, set as a frame would set it:
// at once, each tweak to what it's given.
export function useTold(first: Told): [Told, (some: State) => void] {
  const [state, setState] = useState(first.state);
  return [{ ...first, state }, (some) => setState((was) => ({ ...was, ...some }))];
}

type PageOf = {
  title?: string;
  // Where it stands: live, with none.
  chip?: ReactNode;
  // Its versions, and the one shown.
  versions?: ArtifactVersionsItem[];
  n?: number;
  // What's said of it between its version's line and its tools.
  notes?: ReactNode;
  // How wide its frame's drawn, at first.
  width?: string;
  // What's in its stage, in place of its frame; and beside it.
  stage?: ReactNode;
  tray?: ReactNode;
  tweaks?: ReactNode;
  promoted?: boolean;
};

// Page is an artifact's page from its parts, as ArtifactPage draws it: its
// versions step, its width changes and it stops, and nothing else is
// done.
export function Page({ title = "Where Export goes", chip, versions: all = versions, n: first, notes, width: w = "fit", stage, tray, tweaks, promoted: kept }: PageOf) {
  const [n, setN] = useState(first ?? all[all.length - 1].n);
  const [width, setWidth] = useState(w);
  const [stopped, setStopped] = useState(false);
  const v = all.find((x) => x.n === n) ?? all[0];
  const px = widths.find((x) => x.key === width)?.px ?? 0;
  return (
    <div style={{ height: "100vh" }}>
      <ArtifactView>
        <ArtifactHead>
          <ArtifactTitleLine back={{ to: "/artifacts", label: "Back to Artifacts" }} title={title}>
            {chip ?? (
              <Chip tone={live.tone}>
                <Icon name={live.icon} size={11} />
                {live.words}
              </Chip>
            )}
            <ArtifactTitleRef text={`a1.v${n}`} title="Copy its ref" />
          </ArtifactTitleLine>
          <ArtifactVersionLine n={n} of={all.length} by={v.by === "by you" ? "by you, by hand" : v.by} at={v.at} size="2 KB" kept={v.home ? { repo: v.home.repo, title: `v${n} lives in ${v.home.where}` } : { title: "On this machine: .ws/artifacts/2026-11-20-where-export-goes/" }} />
          {v.note && <ArtifactNote>{v.note}</ArtifactNote>}
          {notes}
          <ArtifactTools>
            <ArtifactVersions versions={all} current={n} onPick={setN} keys={["[", "]"]} />
            <Segmented value={width} onChange={setWidth} aria-label="Width">
              {widths.map((x) => (
                <Segment key={x.key} value={x.key} title={x.title}>
                  {x.label}
                </Segment>
              ))}
            </Segmented>
            <ArtifactActions
              runnable
              source={false}
              stopped={stopped}
              onReload={none}
              onStop={() => setStopped((s) => !s)}
              onSource={none}
              tweaks={tweaks}
              compare={all.length > 1 ? { to: "/artifacts/a1/compare", title: "Side by side: two frames, or their source diff" } : undefined}
              alone="#alone"
              keep={{ promoted: !!kept, onPromote: none, onDiscard: none }}
            />
          </ArtifactTools>
        </ArtifactHead>
        <ArtifactBody>
          <ArtifactStage>{stage ?? <ArtifactFrame src={page(`${title}, v${n}`)} title={`${title}, v${n}`} px={px} frameKey={`a1.v${n}`} />}</ArtifactStage>
          {tray}
        </ArtifactBody>
      </ArtifactView>
    </div>
  );
}
