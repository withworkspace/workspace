import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import { ArtifactFrom } from "./ArtifactFrom";
import { ArtifactRow, artifactRowCodeClass } from "./ArtifactRow";
import { ArtifactsHead } from "./ArtifactsHead";
import { ArtifactsNone } from "./ArtifactsNone";
import { ago, days, discarded, gone, live, promoted } from "./samples";
import { ANYWHERE, type Shown } from "./shown";
import { Text } from "../ui/Text";

// The Artifacts page's list, from its parts: with none yet, with some
// (one from a mission, one from a conversation another machine had, one
// whose folder's gone), with one promoted into a repo and behind its
// latest, and with a filter on that nothing's left under.

const meta: Meta = { title: "Artifacts/List" };
export default meta;

const froms = [
  { from: "export-invoices-as-csv", title: "Export invoices as CSV" },
  { from: "w99", title: "Empty lists, on the other laptop" },
];

function List({ filters = true, shown: first = "all", children }: { filters?: boolean; shown?: Shown; children: ReactNode }) {
  const [shown, setShown] = useState<Shown>(first);
  const [from, setFrom] = useState(ANYWHERE);
  return (
    <div className="page narrow">
      <ArtifactsHead filters={filters} shown={shown} onShown={setShown} discarded={1} froms={froms} from={from} onFrom={setFrom} />
      {children}
    </div>
  );
}

const version = (n: number, of = n) => (
  <Chip tone="quiet" title={`${of} ${of === 1 ? "version" : "versions"}`}>
    v{n}
  </Chip>
);

const here = (id: string) => (
  <>
    <span title="On this machine only: .ws/artifacts/">
      <Icon name="monitor" size={11} /> This machine
    </span>
    <Text mono className={artifactRowCodeClass}>{id}</Text>
  </>
);

const some = (
  <>
    <ArtifactRow to="/artifacts/a4" option selected status={live} title="Empty states" chips={version(3)} note="From the first again: with the way to add one." when={{ by: "by c2", at: ago(6), title: "v3 by c2" }}>
      <span>
        from{" "}
        <span>
          <ArtifactFrom from="w99" title="Empty lists, on the other laptop" />
        </span>
      </span>
      {here("a4")}
    </ArtifactRow>
    <ArtifactRow to="/artifacts/a3" option status={live} title="Export dialog" chips={version(1)} note="The dialog as a page with its files: a stylesheet, a script and the logo." when={{ by: "by c1", at: ago(50), title: "v1 by c1" }}>
      {here("a3")}
    </ArtifactRow>
    <ArtifactRow
      to="/artifacts/a9"
      option
      status={gone}
      title="Lost sketch"
      chips={
        <>
          {version(1)}
          <Chip tone={gone.tone}>{gone.words}</Chip>
        </>
      }
      note="One whose folder goes."
      when={{ by: "by you", at: days(2), title: "v1 by you" }}
    >
      {here("a9")}
    </ArtifactRow>
  </>
);

const kept = (
  <ArtifactRow
    to="/artifacts/a1"
    option
    status={promoted}
    title="Where Export goes"
    chips={
      <>
        {version(2)}
        <Chip tone={promoted.tone}>{promoted.words}</Chip>
      </>
    }
    note="Export under the list, beside the count it exports, so the button says how many."
    when={{ by: "by you", at: days(4), title: "v2 by you" }}
  >
    <span title="v1 lives in records:artifacts/2026-11-20-where-export-goes/, for everyone with records.">
      <Icon name="branch" size={11} /> records · v1
      <Text dim>, v2 here</Text>
    </span>
    <Text mono className={artifactRowCodeClass}>a1</Text>
  </ArtifactRow>
);

export const None: StoryObj = {
  render: () => (
    <List filters={false}>
      <ArtifactsNone />
    </List>
  ),
};

export const Some: StoryObj = {
  render: () => (
    <List>
      <Rows role="listbox" aria-label="Artifacts">
        {some}
      </Rows>
    </List>
  ),
};

export const OnePromoted: StoryObj = {
  render: () => (
    <List>
      <Rows role="listbox" aria-label="Artifacts">
        {some}
        {kept}
      </Rows>
    </List>
  ),
};

export const Discarded: StoryObj = {
  render: () => (
    <List shown="discarded">
      <Rows role="listbox" aria-label="Artifacts">
        <ArtifactRow
          to="/artifacts/a6"
          option
          dimmed
          status={discarded}
          title="Old pager"
          chips={
            <>
              {version(1)}
              <Chip tone={discarded.tone}>{discarded.words}</Chip>
            </>
          }
          note="A pager with numbers. Nobody chose it."
          when={{ by: "by you", at: days(9), title: "v1 by you" }}
        >
          {here("a6")}
        </ArtifactRow>
      </Rows>
    </List>
  ),
};

export const NoneOfThose: StoryObj = {
  render: () => (
    <List shown="promoted">
      <ArtifactsNone shown="promoted" />
    </List>
  ),
};

export const Narrow: StoryObj = {
  render: () => (
    <div style={{ width: 390 }}>
      <List>
        <Rows role="listbox" aria-label="Artifacts">
          {some}
          {kept}
        </Rows>
      </List>
    </div>
  ),
};
