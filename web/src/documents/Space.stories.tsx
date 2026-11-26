import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useRef, useState } from "react";
import { Button, buttonClass } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Markdown } from "../markdown/LazyMarkdown";
import { Rows } from "../ui/Rows";
import { DocumentsActivityFold } from "./DocumentsActivityFold";
import { DocumentsActivitySection } from "./DocumentsActivitySection";
import { DocumentsChangeLine } from "./DocumentsChangeLine";
import { DocumentsChanges } from "./DocumentsChanges";
import { DocumentsEmpty } from "./DocumentsEmpty";
import { DocumentsFlightRow } from "./DocumentsFlightRow";
import { DocumentsFresh } from "./DocumentsFresh";
import { DocumentsInFlightChip } from "./DocumentsInFlightChip";
import { DocumentsSearchBox } from "./DocumentsSearchBox";
import { DocumentsSpaceChange } from "./DocumentsSpaceChange";
import { ago, api, days, records, snippet } from "./samples";
import { SpaceDocRow } from "./SpaceDocRow";
import { SpaceEdge } from "./SpaceEdge";
import { SpaceFrame } from "./SpaceFrame";
import { SpaceHead } from "./SpaceHead";
import { SpaceHitLine } from "./SpaceHitLine";
import { SpaceKindHead } from "./SpaceKindHead";
import { SpaceList } from "./SpaceList";
import { SpaceNewInFolder } from "./SpaceNewInFolder";
import { SpaceOtherHead } from "./SpaceOtherHead";
import { SpacePreview } from "./SpacePreview";
import { SpacePreviewBody } from "./SpacePreviewBody";
import { SpacePreviewChanges } from "./SpacePreviewChanges";
import { SpacePreviewFlight } from "./SpacePreviewFlight";
import { SpacePreviewHead } from "./SpacePreviewHead";
import { SpacePreviewSections } from "./SpacePreviewSections";
import { SpacePreviewSkeleton } from "./SpacePreviewSkeleton";
import { SpaceSplit } from "./SpaceSplit";
import { SpaceTabs } from "./SpaceTabs";
import { Text } from "../ui/Text";

// A space's page, from its parts: its head (up to date, behind and
// pulled, behind with changes of its own, being fetched); its list (a
// search, what's happening folded or open, a tab per kind, a kind's list
// with where each stands, then Other by folder) beside the document
// picked; a search of it, with what it found; a space of folders; and New
// in a folder.

const meta: Meta = { title: "Documents/Space" };
export default meta;

const fresh = <DocumentsFresh behind={0} remote="origin/main" onPull={() => {}} fetched={ago(4)} />;
const newButton = (
  <Button variant="primary">
    <Icon name="plus" /> New <Keys k="n" />
  </Button>
);

function Top({ q: first = "", found, open: opened = false, tabs = true }: { q?: string; found?: string; open?: boolean; tabs?: boolean }) {
  const [q, setQ] = useState(first);
  const [open, setOpen] = useState(opened);
  const ref = useRef<HTMLInputElement>(null);
  return (
    <>
      <DocumentsSearchBox space inputRef={ref} q={q} setQ={setQ} placeholder="Search Records: what its documents say" label="Search Records" onKey={() => {}} found={found ?? "Searching…"} />
      <DocumentsActivityFold inFlight={1} week={3} latest={{ title: "Cache Go modules between jobs", by: "maya", at: ago(40) }} open={open} setOpen={setOpen}>
        <DocumentsActivitySection label="In flight" heading="In flight" count={1} flat>
          <Rows>
            <DocumentsFlightRow capsule="vat-rounding">
              <DocumentsSpaceChange how="changed" name="How VAT is shown on an invoice" path="notes/how-vat-is-shown-on-an-invoice.md" to="#" />
              <DocumentsSpaceChange how="removed" name="An invoice freezes when it's sent" path="adr/0003-an-invoice-freezes-when-it-is-sent.md" />
            </DocumentsFlightRow>
          </Rows>
        </DocumentsActivitySection>
        <DocumentsActivitySection label="Changed lately" heading="Changed lately" count={2} flat>
          <DocumentsChanges>
            <DocumentsChangeLine how="added" name="Cache Go modules between jobs" path="bp/proposed/2026-11-12-cache-go-modules-between-jobs.md" to="#" by="maya" at={ago(40)} />
            <DocumentsChangeLine how="removed" name="Overdue reminders" path="proposals/overdue-reminders.md" by="maya" at={days(2)} />
          </DocumentsChanges>
        </DocumentsActivitySection>
      </DocumentsActivityFold>
      {tabs && (
        <SpaceTabs
          tabs={[
            { id: "adr", label: "ADR", count: q ? "1/3" : "3", empty: false },
            { id: "bp", label: "Build proposal", count: q ? "0/3" : "3", empty: !!q },
            { id: "other", label: "Other", count: "1", empty: false },
          ]}
          value="adr"
          onChange={() => {}}
          label="Jump to a kind"
        />
      )}
    </>
  );
}

function Kind({ title, states, children }: { title: string; states?: [string, number][]; children: ReactNode }) {
  const [stand, setStand] = useState("");
  const [sort, setSort] = useState("made");
  return (
    <section aria-label={title}>
      <SpaceKindHead
        title={title}
        count={3}
        states={states ?? []}
        stand={stand}
        setStand={setStand}
        sorts={[
          ["made", "Newest first"],
          ["title", "By title"],
          ...(states ? [["stands", "By where it stands"] as [string, string]] : []),
          ["changed", "Changed lately"],
        ]}
        sort={sort}
        setSort={setSort}
      />
      <Rows role="listbox" aria-label={title}>
        {children}
      </Rows>
    </section>
  );
}

const preview = (
  <SpacePreview label="Cache Go modules between jobs, previewed">
    <SpacePreviewHead
      chips={
        <>
          <Chip tone="quiet">Build proposal</Chip>
          <Chip tone="quiet">proposed</Chip>
        </>
      }
      made="2026-11-12"
      docRef="records:bp/proposed/2026-11-12-cache-go-modules-between-jobs.md"
      where="records · bp/proposed/2026-11-12-cache-go-modules-between-jobs.md"
      title="Cache Go modules between jobs"
      meta={
        <>
          Changed by maya, 40m ago · 412 words · 2 min read
        </>
      }
      actions={
        <>
          <a href="#" className={buttonClass({ variant: "primary" })}>
            <Icon name="book" size={13} /> Read it <Keys k="⏎" />
          </a>
          <a href="#" className={buttonClass()}>
            <Icon name="pencil" size={13} /> Edit as a page
          </a>
          <a href="#" className={buttonClass({ variant: "ghost" })}>
            <Icon name="layers" size={13} /> In Files
          </a>
        </>
      }
    />
    <SpacePreviewFlight capsules={[{ name: "vat-rounding", to: "#" }]} />
    <SpacePreviewSections
      sections={[
        { anchor: "the-cache", text: "The cache" },
        { anchor: "risks", text: "Risks" },
        { anchor: "what-it-would-take-a-section-with-a-long-name", text: "What it would take, a section with a long name that's cut short" },
      ]}
      onGo={() => {}}
    />
    <SpacePreviewBody>
      <Markdown text={"Every job downloads the modules again. A shared cache would save a minute a job.\n\n## The cache\n\nKeyed by `go.sum`, and kept for a week.\n\n## Risks\n\nA poisoned cache: a job that writes to it can change what every later job builds."} />
    </SpacePreviewBody>
    <SpacePreviewChanges>
      <DocumentsChanges>
        <DocumentsChangeLine how="added" name="Cache Go modules between jobs" path="bp/proposed/2026-11-12-cache-go-modules-between-jobs.md" to="#" subject="Add two build proposals" by="maya" at={ago(40)} />
      </DocumentsChanges>
    </SpacePreviewChanges>
  </SpacePreview>
);

function Page({ head = fresh, top, list, side }: { head?: ReactNode; top: ReactNode; list: ReactNode; side?: ReactNode }) {
  const [width, setWidth] = useState<number>();
  const scroll = useRef<HTMLDivElement>(null);
  return (
    <SpaceFrame>
      <SpaceHead label="Records" color={records.color} count={11} fresh={head} actions={newButton} />
      <SpaceSplit listWidth={width}>
        <SpaceList top={top} scrollRef={scroll} onScroll={() => {}}>
          {list}
        </SpaceList>
        <SpaceEdge width={width} setWidth={setWidth} />
        {side ?? preview}
      </SpaceSplit>
    </SpaceFrame>
  );
}

const lists = (
  <>
    <Kind title="ADR">
      <SpaceDocRow id="lib-1" to="#" title="Money is whole minor units" selected={false} made="0001" changed={{ by: "maya", at: days(9) }} />
      <SpaceDocRow id="lib-2" to="#" title="Webhooks are delivered at least once" selected={false} made="0002" changed={{ by: "tomas lee", at: days(6) }} />
      <SpaceDocRow id="lib-3" to="#" title="An invoice freezes when it's sent" selected={false} made="0003" chips={<DocumentsInFlightChip capsules={["vat-rounding"]} />} changed={{ by: "ines", at: days(5) }} />
    </Kind>
    <Kind
      title="Build proposal"
      states={[
        ["proposed", 2],
        ["accepted", 1],
      ]}
    >
      <SpaceDocRow id="lib-4" to="#" title="Cache Go modules between jobs" selected picked made="2026-11-12" chips={<Chip tone="quiet">proposed</Chip>} changed={{ by: "maya", at: ago(40) }} />
      <SpaceDocRow id="lib-5" to="#" title="A build proposal with a very long title that goes on and on, past where the row has room for it" selected={false} made="2026-11-20" chips={<Chip tone="quiet">proposed</Chip>} changed={{ by: "maya", at: ago(40) }} />
      <SpaceDocRow id="lib-6" to="#" title="Move builds to ARM runners" selected={false} made="2026-10-02" chips={<Chip tone="done">accepted</Chip>} changed={{ by: "maya", at: ago(40) }} />
    </Kind>
    <SpaceOtherHead id="list-other" />
    <section aria-label="Top">
      <GroupHead count={1}>
        <Text mono>(top)</Text>
      </GroupHead>
      <Rows role="listbox" aria-label="Top">
        <SpaceDocRow id="doc-readme" to="#" title="records" selected={false} file="README.md" />
      </Rows>
    </section>
  </>
);

export const WithAKindPicked: StoryObj = {
  render: () => <Page top={<Top />} list={lists} />,
};

export const WhatsHappeningOpen: StoryObj = {
  render: () => <Page top={<Top open />} list={lists} />,
};

export const Searched: StoryObj = {
  render: () => (
    <Page
      top={<Top q="minute" found="1 of 11" />}
      list={
        <Kind title="Build proposal">
          <SpaceDocRow id="lib-4" to="#" title="Cache Go modules between jobs" selected picked made="2026-11-12" hit={<SpaceHitLine heading="The cache" snippet={snippet("A shared cache would save a ", "minute", " a job.")} />} changed={{ by: "maya", at: ago(40) }} />
        </Kind>
      }
    />
  ),
};

export const NothingSearched: StoryObj = {
  render: () => (
    <Page
      top={<Top q="zzqq" found="0 of 11" />}
      list={<DocumentsEmpty>Nothing in Records says that.</DocumentsEmpty>}
      side={
        <SpacePreview none>
          <Text as="p" dim>Nothing found to show.</Text>
        </SpacePreview>
      }
    />
  ),
};

export const Reading: StoryObj = {
  render: () => (
    <Page
      top={<Top />}
      list={lists}
      side={
        <SpacePreview label="Being read">
          <SpacePreviewHead docRef="records:adr/0001-money-is-whole-minor-units.md" where="records · adr/0001-money-is-whole-minor-units.md" made="0001" title="Money is whole minor units" actions={null} />
          <SpacePreviewSkeleton />
        </SpacePreview>
      }
    />
  ),
};

export const Behind: StoryObj = {
  render: () => <Page head={<DocumentsFresh behind={2} remote="origin/main" onPull={() => {}} fetched={ago(4)} />} top={<Top />} list={lists} />,
};

export const BehindWithChangesOfItsOwn: StoryObj = {
  render: () => <Page head={<DocumentsFresh behind={1} remote="origin/main" dirty onPull={() => {}} fetched={ago(4)} />} top={<Top />} list={lists} />,
};

export const Fetching: StoryObj = {
  render: () => <Page head={<DocumentsFresh fetching behind={0} remote="origin/main" onPull={() => {}} />} top={<Top />} list={lists} />,
};

export const OfFolders: StoryObj = {
  render: () => {
    const scroll = { current: null };
    return (
      <SpaceFrame>
        <SpaceHead label="API" color={api.color} folder="docs/" count={5} fresh={fresh} actions={newButton} />
        <SpaceSplit>
          <SpaceList
            scrollRef={scroll}
            onScroll={() => {}}
            top={
              <SpaceTabs
                tabs={[
                  { id: "top", label: "(top)", count: "1", empty: false },
                  { id: "guides", label: "guides/", count: "2", empty: false },
                  { id: "runbooks", label: "runbooks/", count: "1", empty: false },
                ]}
                value="guides"
                onChange={() => {}}
                label="Jump to a folder"
              />
            }
          >
            {(
              [
                ["(top)", [["Docs", "README.md"]]],
                ["guides/", [["Errors", "errors.md"], ["Getting started", "getting-started.md"]]],
                ["runbooks/", [["The queue backs up", "queue-backs-up.md"]]],
              ] as [string, [string, string][]][]
            ).map(([folder, docs]) => (
              <section key={folder} aria-label={folder}>
                <GroupHead count={docs.length}>
                  <Text mono>{folder}</Text>
                </GroupHead>
                <Rows role="listbox" aria-label={folder}>
                  {docs.map(([title, file]) => (
                    <SpaceDocRow key={file} id={file} to="#" title={title} selected={false} picked={file === "errors.md"} file={file} />
                  ))}
                </Rows>
              </section>
            ))}
          </SpaceList>
          <SpaceEdge setWidth={() => {}} />
          <SpacePreview none>
            <Text as="p" dim>Nothing to show.</Text>
          </SpacePreview>
        </SpaceSplit>
      </SpaceFrame>
    );
  },
};

export const NewInAFolder: StoryObj = {
  render: () => (
    <div className="page">
      <SpaceNewInFolder repo="api" label="API" root="docs/" folders={["", "guides/", "runbooks/"]} busy={false} onStart={() => {}} onClose={() => {}} />
    </div>
  ),
};
