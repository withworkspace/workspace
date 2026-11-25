import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef, useState } from "react";
import { Markdown } from "../../markdown/LazyMarkdown";
import { PageHead } from "../../ui/PageHead";
import type { Section } from "../reading";
import { fileShelves, siblings, trail } from "../shelves";
import { Reader, ReaderText } from "./Reader";
import { ReaderCiteList } from "./ReaderCiteList";
import { ReaderCites } from "./ReaderCites";
import { ReaderContents } from "./ReaderContents";
import { ReaderCrumbs } from "./ReaderCrumbs";
import { ReaderEarlier } from "./ReaderEarlier";
import { ReaderFilePath } from "./ReaderFilePath";
import { ReaderFolder } from "./ReaderFolder";
import { ReaderMeta } from "./ReaderMeta";
import { ReaderPager } from "./ReaderPager";
import { ReaderPill } from "./ReaderPill";
import { ReaderRail } from "./ReaderRail";
import { ReaderTalks } from "./ReaderTalks";

// The document reader, from its parts: a document read with its contents
// beside it (the section you're in marked, those passed less faint), what's
// said of it under its title in each state it's in, its path with a
// folder's menu open, the documents either side of it, what cites it and
// the conversations about it; and where the rail doesn't fit, the pill and
// the contents it opens.

const meta: Meta = { title: "Files/Reader" };
export default meta;

const none = () => {};
const at = "2026-11-24T10:00:00Z";

const sections: Section[] = [
  { anchor: "invoices", level: 2, text: "Invoices" },
  { anchor: "sorting", level: 3, text: "Sorting" },
  { anchor: "money", level: 3, text: "Money" },
  { anchor: "reports", level: 2, text: "Reports" },
  { anchor: "theme", level: 2, text: "Theme" },
  { anchor: "dark-mode", level: 3, text: "Dark mode" },
  { anchor: "releasing", level: 2, text: "Releasing" },
];

const paths = ["README.md", "docs/guide.md", "docs/draft.md", "docs/guides/setup.md", "docs/guides/testing.md", "docs/guides/deploys.md"];
const here = { path: "docs/guides/testing.md", title: "Testing" };
const shelves = fileShelves("web/fx", paths, here);
const on = trail(shelves, here.path);
const sib = siblings(shelves, here.path)!;

const text = ["How the dashboard is put together, and where to change it.", ...sections.map((s) => `${s.level === 2 ? "##" : "###"} ${s.text}\n\nSome words about ${s.text.toLowerCase()}.`)].join("\n\n");

const meta_ = (more: Partial<Parameters<typeof ReaderMeta>[0]> = {}) => (
  <ReaderMeta repo="web" worktree=".ground" ground flights={[]} docRef="web:docs/guide.md" onAsk={none} onEdit={none} filesTo="/" {...more} />
);

function Text() {
  return (
    <>
      <PageHead title="guide.md" />
      {meta_()}
      <ReaderText>
        <Markdown text={text} />
      </ReaderText>
      <ReaderFilePath path="docs/guide.md" />
      <ReaderPager sib={sib} to={() => "/"} />
    </>
  );
}

export const WithItsContentsBeside: StoryObj = {
  render: () => (
    <Reader rail after={<ReaderRail sections={sections} reading={{ at: 3, pct: 46, past: true }} hidden={false} go={none} />}>
      <ReaderCrumbs from={{ to: "/", label: "Repos" }} on={on} title="Testing" toggle={none} />
      <Text />
    </Reader>
  ),
};

export const ItsContentsHidden: StoryObj = {
  render: () => (
    <Reader rail after={<ReaderRail sections={sections} reading={{ at: 0, pct: 4, past: false }} hidden go={none} />}>
      <Text />
    </Reader>
  ),
};

export const RailWithNoSections: StoryObj = {
  render: () => (
    <Reader rail after={<ReaderRail sections={[]} reading={{ at: -1, pct: 0, past: false }} hidden={false} go={none} />}>
      <Text />
    </Reader>
  ),
};

// Where the rail doesn't fit: the pill, stuck to the top once you've
// scrolled past the title, and the contents it opens under it.
export const FoldedIntoAPill: StoryObj = {
  render: () => {
    const pill = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState(true);
    return (
      <div style={{ maxWidth: 520, paddingTop: 24 }}>
        <Reader folded after={open && <ReaderContents title="The dashboard guide" sections={sections} at={2} anchor={() => pill.current} go={none} copy={none} close={() => setOpen(false)} />}>
          <ReaderPill title="The dashboard guide" sections={sections} reading={{ at: 2, pct: 31, past: true }} open={open} toggle={() => setOpen(!open)} pill={pill} />
          <Text />
        </Reader>
      </div>
    );
  },
};

// What's said of a document under its title: a space's ADR, edited in a
// page, in flight in two capsules (one in a pull request), its contents
// folded into a button; one read in a capsule; one that can't be edited
// here.
export const Meta_: StoryObj = {
  name: "Meta",
  render: () => (
    <div className="page">
      {meta_({
        kind: { title: "ADR", hint: "A decision, and why" },
        page: { name: "p8", to: "/" },
        flights: [
          { capsule: "tax-rounding", pr: 42, to: "/" },
          { capsule: "pdf-vat-rows", to: "/" },
        ],
        contents: { open: false, onToggle: none },
      })}
      {meta_({ worktree: "fx", ground: false, docRef: "web/fx:docs/guide.md" })}
      {meta_({ editUnavailable: "it's not markdown" })}
    </div>
  ),
};

// Its path, with the menu of what's on one of its shelves open: filter
// it, step onto a shelf, go up.
export const PathWithAFolderOpen: StoryObj = {
  render: () => {
    const crumbs = useRef<HTMLElement>(null);
    const column = useRef<HTMLDivElement>(null);
    const [folder, setFolder] = useState<string | undefined>(on[on.length - 1]?.id);
    return (
      <Reader
        columnRef={column}
        after={
          folder !== undefined && (
            <ReaderFolder
              key={folder}
              shelves={shelves}
              id={folder}
              here={here.path}
              on={on}
              anchor={() => crumbs.current?.querySelector<HTMLElement>(`[data-folder-crumb="${CSS.escape(folder)}"]`) ?? crumbs.current}
              column={() => column.current}
              openIn={() => ({ to: "/", label: "Open in Files" })}
              go={none}
              step={setFolder}
              close={() => setFolder(undefined)}
            />
          )
        }
      >
        <ReaderCrumbs from={{ to: "/", label: "Repos" }} on={on} title="Testing" open={folder} toggle={(id) => setFolder(folder === id ? undefined : id)} nav={crumbs} />
        <div style={{ height: 420 }} />
      </Reader>
    );
  },
};

// The documents either side of it: the first and the last have nothing
// on one side.
export const Pager: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16 }}>
      <ReaderPager sib={sib} to={() => "/"} />
      <ReaderPager sib={{ ...sib, prev: undefined, at: 1 }} to={() => "/"} />
      <ReaderPager sib={{ ...sib, next: undefined, at: sib.of }} to={() => "/"} />
    </div>
  ),
};

// Under its text: what cites it and what it cites, the conversations
// about it (one answering, one yours to answer), threads from before it
// was edited as a page, and where its file is.
export const UnderItsText: StoryObj = {
  render: () => (
    <div className="page">
      <ReaderEarlier count={1} docRef="records:adr/0001-money-is-whole-minor-units.md" />
      <ReaderEarlier count={3} docRef="records:adr/0001-money-is-whole-minor-units.md" />
      <ReaderCites label="Cited by, and cites">
        <ReaderCiteList
          title="Cited by"
          items={[
            { key: "a", to: "/", title: "Invoices freeze when they're sent", file: "0003-an-invoice-freezes.md", line: 12, how: "link" },
            { key: "b", to: "/", title: "How VAT is shown on an invoice", file: "how-vat-is-shown.md", line: 4, how: "name" },
          ]}
        />
        <ReaderCiteList title="It cites" items={[{ key: "c", to: "/", title: "Webhooks are delivered at least once", file: "0002-webhooks.md", line: 30, how: "link" }]} />
      </ReaderCites>
      <ReaderCites label="Conversations about it">
        <ReaderTalks
          items={[
            { id: "w1", to: "/", title: "Why minor units?", status: { icon: "spinner", tone: "progress", words: "Answering", live: true }, sub: "“Money is kept in whole minor units” in § Decision", at },
            { id: "w2", to: "/", title: "What about currencies with three decimals?", status: { icon: "reply", tone: "attention", words: "Your turn" }, at },
            { id: "w3", to: "/", title: "Rounding", status: { icon: "check-circle", tone: "quiet", words: "Resolved" }, sub: "§ Consequences", at },
          ]}
        />
      </ReaderCites>
      <ReaderFilePath path="adr/0001-money-is-whole-minor-units.md" />
    </div>
  ),
};
