import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { DiffFile } from "../api";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Segment, Segmented } from "../ui/Segmented";
import { ArtifactStage } from "./ArtifactBody";
import { ArtifactCompareBody } from "./ArtifactCompareBody";
import { ArtifactCompareDiff, ArtifactCompareDiffCount } from "./ArtifactCompareDiff";
import { ArtifactCompareFile } from "./ArtifactCompareFile";
import { ArtifactCompareSide } from "./ArtifactCompareSide";
import { ArtifactFrame } from "./ArtifactFrame";
import { ArtifactHead } from "./ArtifactHead";
import { ArtifactNote } from "./ArtifactNote";
import { ArtifactTitleLine } from "./ArtifactTitleLine";
import { ArtifactTools, ArtifactToolsEnd } from "./ArtifactTools";
import { ArtifactTweaksButton } from "./ArtifactTweaksButton";
import { ArtifactVersionLine } from "./ArtifactVersionLine";
import { ArtifactVersions } from "./ArtifactVersions";
import { ArtifactView } from "./ArtifactView";
import { FileDiff } from "./FileDiff";
import { none, page, suggested, told, useTold, versions } from "./samples";
import { widths } from "./shown";
import { TweaksBody } from "./TweaksBody";
import { TweaksHead } from "./TweaksHead";

// Comparing, from its parts: two versions side by side, each side's head
// over its frame; their source diff, side by side (split) and unified,
// each file that differs under its path; and a narrow window's, one side
// at a time.

const meta: Meta = { title: "Artifacts/Compare" };
export default meta;

const choices = [
  { id: "a1", title: "Where Export goes" },
  { id: "a2", title: "Plan list" },
  { id: "a3", title: "Export dialog" },
];

function Side({ i, n, onPick }: { i: 0 | 1; n: number; onPick: (n: number) => void }) {
  const v = versions.find((x) => x.n === n) ?? versions[0];
  const [said, set] = useTold({ ...told, state: i ? { ...told.state, ...suggested } : told.state });
  return (
    <ArtifactCompareSide
      label={i ? "The right one's artifact" : "The left one's artifact"}
      picked="a1"
      choices={choices}
      onPick={none}
      versions={<ArtifactVersions versions={versions} current={n} onPick={onPick} keys={i ? ["[", "]"] : ["{", "}"]} />}
      tweaks={
        <ArtifactTweaksButton label={i ? "The right one's tweaks" : "The left one's tweaks"} changed={i ? 3 : 0}>
          <TweaksHead told={said} />
          <TweaksBody told={said} set={set} />
        </ArtifactTweaksButton>
      }
      alone="#alone"
    >
      <ArtifactVersionLine n={n} of={versions.length} by={v.by} at={v.at} size="2 KB" kept={v.home ? { repo: v.home.repo, title: v.home.where } : { title: "On this machine" }} />
      {v.note && <ArtifactNote dim>{v.note}</ArtifactNote>}
    </ArtifactCompareSide>
  );
}

const index: DiffFile = {
  path: "index.html",
  status: "modified",
  additions: 3,
  deletions: 2,
  hunks: [
    {
      oldStart: 1,
      oldLines: 6,
      newStart: 1,
      newLines: 7,
      lines: [
        { k: "c", o: 1, n: 1, t: "<!doctype html>" },
        { k: "c", o: 2, n: 2, t: '<main class="list">' },
        { k: "d", o: 3, t: '  <nav class="toolbar"><button>Filter</button><button>Export</button></nav>' },
        { k: "a", n: 3, t: '  <nav class="toolbar"><button>Filter</button></nav>' },
        { k: "c", o: 4, n: 4, t: '  <ul id="invoices"></ul>' },
        { k: "d", o: 5, t: "</main>" },
        { k: "a", n: 5, t: '  <footer><span id="count">12 invoices</span> <button>Export 12</button></footer>' },
        { k: "a", n: 6, t: "</main>" },
        { k: "c", o: 6, n: 7, t: '<script src="app.js"></script>' },
      ],
    },
  ],
} as DiffFile;

const added: DiffFile = { path: "style.css", status: "added", additions: 2, deletions: 0, hunks: [{ oldStart: 0, oldLines: 0, newStart: 1, newLines: 2, lines: [{ k: "a", n: 1, t: "footer { display: flex; gap: 8px; }" }, { k: "a", n: 2, t: "#count { color: #666; }" }] }] } as DiffFile;

const logo: DiffFile = { path: "img/logo.png", status: "modified", binary: true, additions: 0, deletions: 0, hunks: [] };

function Compare({ diff: first = false, split: firstSplit = true, narrow = false }: { diff?: boolean; split?: boolean; narrow?: boolean }) {
  const [diff, setDiff] = useState(first);
  const [split, setSplit] = useState(firstSplit);
  const [width, setWidth] = useState("fit");
  const [shown, setShown] = useState<0 | 1>(1);
  const [ns, setNs] = useState<[number, number]>([1, 2]);
  const px = widths.find((x) => x.key === width)?.px ?? 0;
  const sides: (0 | 1)[] = narrow ? [shown] : [0, 1];
  return (
    <div style={{ height: "100vh", width: narrow ? 390 : undefined }}>
      <ArtifactView>
        <ArtifactHead>
          <ArtifactTitleLine back={{ to: "/artifacts/a1", label: "Back to Where Export goes" }} title="Where Export goes">
            <Chip tone="quiet">
              <Icon name="split" size={11} />
              {`v${ns[0]} and v${ns[1]}`}
            </Chip>
          </ArtifactTitleLine>
          <ArtifactTools>
            <Segmented value={diff ? "diff" : "frames"} onChange={(v) => setDiff(v === "diff")} aria-label="What's compared">
              <Segment value="frames" title="Each running in its frame (d)">
                <Icon name="monitor" size={13} /> Frames
              </Segment>
              <Segment value="diff" title="Their files, the left's against the right's (d)">
                <Icon name="code" size={13} /> Source diff
              </Segment>
            </Segmented>
            {diff ? (
              !narrow && (
                <Segmented value={split ? "split" : "unified"} onChange={(v) => setSplit(v === "split")} aria-label="The diff">
                  <Segment value="unified" title="Unified (u)">
                    <Icon name="unified" size={13} /> Unified
                  </Segment>
                  <Segment value="split" title="Side by side (u)">
                    <Icon name="split" size={13} /> Split
                  </Segment>
                </Segmented>
              )
            ) : (
              <Segmented value={width} onChange={setWidth} aria-label="Width">
                {widths.map((w) => (
                  <Segment key={w.key} value={w.key} title={`${w.title}, both`}>
                    {w.label}
                  </Segment>
                ))}
              </Segmented>
            )}
            {narrow && (
              <Segmented value={shown ? "right" : "left"} onChange={(v) => setShown(v === "right" ? 1 : 0)} aria-label="Which one's shown">
                <Segment value="left">a1.v{ns[0]}</Segment>
                <Segment value="right">a1.v{ns[1]}</Segment>
              </Segmented>
            )}
            <ArtifactToolsEnd>
              <Button variant="ghost" size="small" onClick={() => setNs(([l, r]) => [r, l])} title="Each where the other was: the diff the other way about">
                <Icon name="swap" size={13} /> Swap <Keys k="S" />
              </Button>
            </ArtifactToolsEnd>
          </ArtifactTools>
        </ArtifactHead>
        <ArtifactCompareBody one={sides.length === 1}>
          {sides.map((i) => (
            <Side key={`head${i}`} i={i} n={ns[i]} onPick={(n) => setNs((was) => (i ? [was[0], n] : [n, was[1]]))} />
          ))}
          {diff ? (
            <ArtifactCompareDiff>
              <ArtifactCompareDiffCount files={3} a={`a1.v${ns[0]}`} b={`a1.v${ns[1]}`} />
              <FileDiff file={index} lines={index.hunks[0].lines.filter((l) => l.k !== "d").map((l) => l.t)} a={`a1.v${ns[0]}`} b={`a1.v${ns[1]}`} split={split && !narrow} named />
              <FileDiff file={added} lines={added.hunks[0].lines.map((l) => l.t)} a={`a1.v${ns[0]}`} b={`a1.v${ns[1]}`} split={split && !narrow} named />
              <ArtifactCompareFile path={logo.path} status={logo.status} binary deletions={0} additions={0} a={`a1.v${ns[0]}`} b={`a1.v${ns[1]}`} />
            </ArtifactCompareDiff>
          ) : (
            sides.map((i) => (
              <ArtifactStage key={`frame${i}`} compared>
                <ArtifactFrame src={page(`Where Export goes, v${ns[i]}`, i ? "#eef3f1" : "#f6f5f2")} title={`Where Export goes, v${ns[i]}`} px={px} frameKey={`a1.v${ns[i]}`} />
              </ArtifactStage>
            ))
          )}
        </ArtifactCompareBody>
      </ArtifactView>
    </div>
  );
}

export const SideBySide: StoryObj = { render: () => <Compare /> };
export const SourceDiffSplit: StoryObj = { render: () => <Compare diff /> };
export const SourceDiffUnified: StoryObj = { render: () => <Compare diff split={false} /> };
export const OneAtATime: StoryObj = { render: () => <Compare narrow /> };
