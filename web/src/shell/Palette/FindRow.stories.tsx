import type { Meta, StoryObj } from "@storybook/react-vite";
import { Icon } from "../../ui/Icon";
import { finderBox } from "../../ui/Finder";
import { FindCodeRow } from "./FindCodeRow";
import { FindMore } from "./FindMore";
import { FindNone } from "./FindNone";
import { FindNote } from "./FindNote";
import { FindRow } from "./FindRow";
import { FindSection } from "./FindSection";
import { FindTitle } from "./FindTitle";
import { files, hits, missions, places, pulls, type SampleRow } from "./samples";
import { titleParts } from "./title";

// Find's rows, each kind in a box as wide as Find's list and in one too
// narrow for them: a row with a status or an icon, with the letters that
// matched, a hint, notes in a tone and its kind; a path, read from its
// end where it doesn't fit; a line a file says; and what's said under
// rows, and with none. The second of each group has the cursor on it.

const meta: Meta = { title: "Find/FindRow" };
export default meta;

const row = (r: SampleRow, on: boolean, kind = false) => (
  <FindRow key={r.id} id={`story-${r.id}`} on={on} mark={r.mark} title={<FindTitle parts={titleParts(r.title, r.at ?? [], r.path)} path={r.path} />} hint={r.hint} notes={r.notes} tone={r.tone} kind={kind ? r.kind : undefined} />
);

function Rows({ width }: { width: number }) {
  return (
    <div className={finderBox} style={{ width, padding: "4px 6px 6px" }}>
      <FindSection label="Missions" title="Missions, each saying its kind">
        {missions.map((r, i) => row(r, i === 1, true))}
        <FindMore count={4} narrows />
      </FindSection>
      <FindSection label="Pull requests" title="Pull requests, with notes">
        {pulls.map((r, i) => row(r, i === 1))}
        <FindMore count={2} />
      </FindSection>
      <FindSection label="Files" title="Files: paths">
        {files.map((r, i) => row(r, i === 1))}
        <FindNote>What capsules have changed comes first.</FindNote>
      </FindSection>
      <FindSection label="Do" title="Things to do, and places">
        {places.map((r, i) => row(r, i === 1))}
        <FindRow id="story-bare" mark={<Icon name="chevron-right" />} title={<FindTitle parts={titleParts("A row with only a name", [])} />} />
      </FindSection>
      <FindSection label="Code" title="Lines the files say">
        {hits.map((h, i) => (
          <FindCodeRow key={i} id={`story-code-${i}`} on={i === 1} h={h} />
        ))}
        <FindNote capped>The first 200 lines that say it. Type more to narrow it.</FindNote>
      </FindSection>
      <FindSection label="Nothing" title="With nothing found">
        <FindNone>Nothing matches “zzqq”.</FindNone>
      </FindSection>
    </div>
  );
}

export const Rows_: StoryObj = {
  name: "Rows",
  render: () => (
    <div className="page" style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "start" }}>
      <Rows width={640} />
      <Rows width={340} />
    </div>
  ),
};
