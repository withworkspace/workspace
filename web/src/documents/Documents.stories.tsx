import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef, useState } from "react";
import { Button } from "../ui/Button";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { DocumentsAcross } from "./DocumentsAcross";
import { DocumentsActivitySection } from "./DocumentsActivitySection";
import { DocumentsChangeLine } from "./DocumentsChangeLine";
import { DocumentsChanges } from "./DocumentsChanges";
import { DocumentsEmpty } from "./DocumentsEmpty";
import { DocumentsFlightRow } from "./DocumentsFlightRow";
import { DocumentsFrame } from "./DocumentsFrame";
import { DocumentsGrid } from "./DocumentsGrid";
import { DocumentsHitRow } from "./DocumentsHitRow";
import { DocumentsSearch } from "./DocumentsSearch";
import { DocumentsSpaceCard } from "./DocumentsSpaceCard";
import { DocumentsSpaceCardFoot } from "./DocumentsSpaceCardFoot";
import { DocumentsSpaceCardKind } from "./DocumentsSpaceCardKind";
import { DocumentsSpaceCardKinds } from "./DocumentsSpaceCardKinds";
import { DocumentsSpaceCards } from "./DocumentsSpaceCards";
import { DocumentsSpaceChange } from "./DocumentsSpaceChange";
import { DocumentsSpaceTag } from "./DocumentsSpaceTag";
import { DocumentsSub } from "./DocumentsSub";
import { ago, api, days, records, snippet } from "./samples";

// Documents, from its parts: what's happening across the spaces over a
// card for each (one with kinds, where they stand and which changed last;
// one of folders, behind its remote; one still being read; one not
// cloned), with Local beside them; and a search, with what it found, and
// with nothing found.

const meta: Meta = { title: "Documents/Documents" };
export default meta;

const head = (
  <PageHead
    title="Documents"
    actions={
      <>
        <Button>
          New as… <Keys k="N" />
        </Button>
        <Button variant="primary">
          <Icon name="plus" /> New page <Keys k="n" />
        </Button>
      </>
    }
  />
);

function Search({ q: first = "" }: { q?: string }) {
  const [q, setQ] = useState(first);
  const [kind, setKind] = useState("");
  const [state, setState] = useState("");
  const ref = useRef<HTMLInputElement>(null);
  return (
    <DocumentsSearch
      inputRef={ref}
      q={q}
      setQ={setQ}
      onKey={() => {}}
      kinds={[
        { name: "adr", title: "ADR" },
        { name: "bp", title: "Build proposal" },
      ]}
      kind={kind}
      setKind={setKind}
      states={kind === "bp" ? ["proposed", "accepted"] : []}
      state={state}
      setState={setState}
    />
  );
}

const lately = (
  <DocumentsActivitySection label="Changed lately" heading={<>Changed lately<DocumentsSub> · across 2 spaces</DocumentsSub></>} count={7}>
    <DocumentsChanges more="and 2 earlier changes">
      <DocumentsChangeLine how="added" name="Cache Go modules between jobs" path="bp/proposed/2026-11-12-cache-go-modules-between-jobs.md" to="#" subject="Add two build proposals" more={{ count: 2, title: "+ Move builds to ARM runners\n+ A build proposal" }} space={<DocumentsSpaceTag {...records} />} by="maya" at={ago(40)} />
      <DocumentsChangeLine how="changed" name="Getting started" path="docs/guides/getting-started.md" to="#" space={<DocumentsSpaceTag {...api} />} by="tomas lee" at={ago(300)} />
      <DocumentsChangeLine how="removed" name="Overdue reminders" path="proposals/overdue-reminders.md" subject="Drop the overdue reminders proposal, which nobody's picked up since the spring" space={<DocumentsSpaceTag {...records} />} by="maya" at={days(2)} />
    </DocumentsChanges>
  </DocumentsActivitySection>
);

const inFlight = (
  <DocumentsActivitySection label="In flight" heading="In flight" count={2} title="Capsules with changes to the documents">
    <Rows>
      <DocumentsFlightRow capsule="vat-rounding" mission="Round VAT per row" pr={{ number: 41, title: "Round VAT per row", to: "#" }} space={<DocumentsSpaceTag {...records} end />}>
        <DocumentsSpaceChange how="changed" name="How VAT is shown on an invoice" path="notes/how-vat-is-shown-on-an-invoice.md" to="#" />
        <DocumentsSpaceChange how="added" name="Round VAT per row" path="proposals/round-vat-per-row.md" to="#" />
        <DocumentsSpaceChange how="removed" name="An invoice freezes when it's sent" path="adr/0003-an-invoice-freezes-when-it-is-sent.md" />
      </DocumentsFlightRow>
      <DocumentsFlightRow capsule="runbooks" space={<DocumentsSpaceTag {...api} end />}>
        <DocumentsSpaceChange how="added" name="The queue backs up" path="docs/runbooks/queue-backs-up.md" to="#" />
      </DocumentsFlightRow>
    </Rows>
  </DocumentsActivitySection>
);

const cards = (few: boolean) => (
  <DocumentsSpaceCards few={few}>
    <DocumentsSpaceCard id="space-records" to="#" label="Records" color={records.color} repo="records-repo" count="11 documents" selected={false} onHover={() => {}} foot={<DocumentsSpaceCardFoot stands={{ fetched: ago(12) }} inFlight={1} drafts={1} />}>
      <DocumentsSpaceCardKinds>
        <DocumentsSpaceCardKind to="#" name="ADR" count={3} last={{ title: "An invoice freezes when it's sent", at: days(5) }} />
        <DocumentsSpaceCardKind to="#" name="Build proposal" count={3} last={{ title: "A build proposal with a very long title that goes on and on, past where the row has room", at: days(1) }} stands="2 proposed · 1 accepted" />
        <DocumentsSpaceCardKind to="#" name="Other" count={1} />
      </DocumentsSpaceCardKinds>
    </DocumentsSpaceCard>
    <DocumentsSpaceCard id="space-api" to="#" label="API" color={api.color} folder="docs/" count="5 documents" selected onHover={() => {}} foot={<DocumentsSpaceCardFoot stands={{ behind: 3 }} inFlight={0} drafts={2} />}>
      <DocumentsSpaceCardKinds>
        <DocumentsSpaceCardKind to="#" name="(top)" folder count={1} />
        <DocumentsSpaceCardKind to="#" name="guides/" folder count={2} last={{ title: "Getting started", at: ago(300) }} />
        <DocumentsSpaceCardKind to="#" name="runbooks/" folder count={2} />
      </DocumentsSpaceCardKinds>
    </DocumentsSpaceCard>
    {!few && (
      <>
        <DocumentsSpaceCard id="space-web" to="#" label="Web" color="#d787ff" count="9 documents" selected={false} onHover={() => {}} foot={<DocumentsSpaceCardFoot stands={undefined} inFlight={0} drafts={0} />}>
          <DocumentsSpaceCardKinds reading="design 6 · notes 3" />
        </DocumentsSpaceCard>
        <DocumentsSpaceCard id="space-handbook" to="#" label="Handbook" color="#ffaf00" count="Not cloned" selected={false} onHover={() => {}} foot={<DocumentsSpaceCardFoot stands={{ cloned: false, repo: "handbook" }} inFlight={0} drafts={0} />} />
      </>
    )}
  </DocumentsSpaceCards>
);

// Local's rows are the pages' own (pages/Pages.tsx's PageRow): here,
// Local with none.
const local = (
  <>
    <GroupHead title="Pages written here, on this machine, that aren't in a space">
      Local <Count>0</Count>
    </GroupHead>
    <DocumentsEmpty>
      No pages here. A page is somewhere to write a problem down and work it out:{" "}
      <Button variant="link" onClick={() => {}}>
        start one
      </Button>
      .
    </DocumentsEmpty>
  </>
);

export const TwoSpaces: StoryObj = {
  render: () => (
    <DocumentsFrame>
      {head}
      <Search />
      <DocumentsAcross>
        {lately}
        {inFlight}
      </DocumentsAcross>
      <DocumentsGrid
        spaces={
          <>
            <GroupHead count={2}>Spaces</GroupHead>
            {cards(true)}
          </>
        }
        local={local}
      />
    </DocumentsFrame>
  ),
};

export const ManySpaces: StoryObj = {
  render: () => (
    <DocumentsFrame>
      {head}
      <Search />
      <DocumentsGrid
        spaces={
          <>
            <GroupHead count={4}>Spaces</GroupHead>
            {cards(false)}
          </>
        }
        local={local}
      />
    </DocumentsFrame>
  ),
};

export const Found: StoryObj = {
  render: () => (
    <DocumentsFrame>
      {head}
      <Search q="invoice" />
      <section aria-label="Found">
        <GroupHead count={3}>Found</GroupHead>
        <Rows role="listbox" aria-label="Documents found">
          <DocumentsHitRow id="hit-0" to="#" selected onHover={() => {}} onOpen={(e) => e.preventDefault()} title="An invoice freezes when it's sent" heading="Decision" state="accepted" every snippet={snippet("Once an ", "invoice", " is sent, its rows can't change: a correction is a credit note.")} where="Records" />
          <DocumentsHitRow id="hit-1" to="#" selected={false} onHover={() => {}} onOpen={(e) => e.preventDefault()} title="How VAT is shown on an invoice" every={false} snippet={snippet("Each row of an ", "invoice", " shows its VAT, and the total shows the sum.")} where="Records" />
          <DocumentsHitRow id="hit-2" to="#" selected={false} onHover={() => {}} onOpen={(e) => e.preventDefault()} title="Errors" every snippet={[]} where="API · docs/" />
        </Rows>
        <DocumentsEmpty>12 more: say more of what you're after.</DocumentsEmpty>
      </section>
    </DocumentsFrame>
  ),
};

export const NothingFound: StoryObj = {
  render: () => (
    <DocumentsFrame>
      {head}
      <Search q="zzqq" />
      <DocumentsEmpty>No documents say that.</DocumentsEmpty>
    </DocumentsFrame>
  ),
};
