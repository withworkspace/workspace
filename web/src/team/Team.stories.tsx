import type { Meta, StoryObj } from "@storybook/react-vite";
import { Fragment, useState } from "react";
import type { TeamItem } from "../api";
import { Button } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";
import { Keys } from "../ui/Keys";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { Time } from "../ui/Time";
import { TeamAsOf } from "./TeamAsOf";
import { TeamChange } from "./TeamChange";
import { TeamFilter } from "./TeamFilter";
import { TeamGroup } from "./TeamGroup";
import { TeamNarrative, TeamNarrativeNew, TeamNarrativeReading } from "./TeamNarrative";
import { TeamSeenLine } from "./TeamSeenLine";
import { TeamSub } from "./TeamSub";

// The Team page, from its parts: its head (when it last looked, its
// filter), the line of what's going on, a day's changes in each state a
// row can be in (new since you looked, being read, worth knowing, near a
// capsule of yours, in several repos, opened on what happened to it, one
// whose gist couldn't be written), with the rule under what's new, and the
// same changes by person.

const meta: Meta = { title: "Team/Team" };
export default meta;

const none = () => {};
const at = (hours: number) => new Date(Date.now() - hours * 3_600_000).toISOString();
const repos: Record<string, { label: string; color?: string }> = {
  api: { label: "api", color: "#6a8fd8" },
  web: { label: "web", color: "#d88a6a" },
  records: { label: "records", color: "#7bb87b" },
};
const repoOf = (r: string) => repos[r] ?? { label: r };
const people: Record<string, string> = { ana: "Ana", james: "James", claude: "Claude" };
const who = (keys: string[]) => keys.map((k) => people[k] ?? k);

const items: TeamItem[] = [
  {
    key: "i1",
    strand: "s1",
    kind: "pr",
    repo: "api",
    pr: 41,
    name: "Limit each key to 100 requests a minute",
    state: "in-review",
    people: ["ana", "james"],
    words: "Ana opened it, James asked for changes",
    at: at(1),
    additions: 212,
    deletions: 18,
    gist: "Counts each key's requests in a sliding window and answers `429` past the limit.",
    know: [
      { kind: "contract", file: "api/limits.go", text: "A request over the limit is answered `429` with `Retry-After`." },
      { kind: "action", file: "docs/limits.md", text: "Clients that retry at once should read `Retry-After`." },
    ],
    events: [
      { id: "e1", at: at(5), kind: "opened", repo: "api", title: "", who: "ana" },
      { id: "e2", at: at(1), kind: "reviewed", repo: "api", title: "", who: "james", verdict: "changes" },
    ],
  },
  {
    key: "i2",
    strand: "s2",
    kind: "commits",
    repo: "web",
    name: "Show VAT by rate on the PDF",
    state: "landed",
    people: ["james"],
    words: "James landed 3 commits",
    at: at(2),
    additions: 64,
    deletions: 9,
    narrative: "writing",
    touches: [{ name: "lint", title: "Lint settings", files: ["web/.eslintrc"] }],
    near: [{ capsule: "web/invoice-pdf", files: ["web/src/invoices/pdf.ts", "web/src/invoices/vat.ts", "web/src/invoices/money.ts"] }],
    events: [{ id: "e3", at: at(2), kind: "landed", repo: "web", title: "invoices: VAT by rate", commit: "8a1f3c2d9e", who: "james", branch: "main", paths: ["web/src/invoices/pdf.ts", "web/src/invoices/vat.ts", "web/src/invoices/money.ts", "web/src/invoices/pdf.test.ts"], files: 4 }],
  },
  {
    key: "i3",
    strand: "s3",
    kind: "pr",
    repo: "api",
    repos: ["api", "web", "records"],
    prs: [
      { repo: "api", number: 44, name: "Export invoices as CSV", state: "landed", url: "https://github.com/sorrel/api/pull/44" },
      { repo: "web", number: 120, name: "Export invoices as CSV", state: "in-review" },
      { repo: "records", number: 9, name: "Export invoices as CSV", state: "in-progress" },
    ],
    name: "Export invoices as CSV",
    state: "in-review",
    people: ["claude", "ana"],
    words: "Claude opened 3 pull requests",
    at: at(3),
    additions: 340,
    deletions: 41,
    gist: "Adds an export of a month's invoices as CSV, from the API to the invoices page.",
    events: [
      { id: "e4", at: at(4), kind: "opened", repo: "api", title: "", who: "claude" },
      { id: "e5", at: at(3), kind: "merged", repo: "api", title: "", who: "ana" },
    ],
  },
  {
    key: "i4",
    strand: "s4",
    kind: "pr",
    repo: "records",
    pr: 8,
    name: "ADR 0002: invoices are immutable once sent",
    state: "closed",
    people: ["ana"],
    words: "Ana closed it",
    at: at(6),
    narrative: "failed",
    gistError: "the agent ran out of time",
    events: [{ id: "e6", at: at(6), kind: "closed", repo: "records", title: "" }],
  },
];

const row = (it: TeamItem, o: { open?: boolean; selected?: boolean; fresh?: boolean; dated?: boolean } = {}) => (
  <TeamChange
    key={it.key}
    id={it.key}
    item={it}
    names={who(it.people)}
    repoOf={repoOf}
    selected={!!o.selected}
    open={!!o.open}
    fresh={!!o.fresh}
    dated={!!o.dated}
    know
    who={(e) => (e.who ? people[e.who] : undefined)}
    onHover={none}
    onToggle={none}
    onOpen={none}
    onWrite={none}
  />
);

export const Head: StoryObj = {
  render: function Render() {
    const [q, setQ] = useState("");
    return (
      <div className="page">
        <PageHead
          title="Team"
          actions={
            <>
              <TeamAsOf at={at(0.1)} />
              <TeamFilter value={q} onChange={setQ} />
            </>
          }
        />
        <PageHead title="Team, looking" actions={<TeamAsOf at={at(0.1)} looking />} />
        <TeamNarrative dim>
          <TeamNarrativeNew>
            2 new since you looked <Time iso={at(3)} as="relative" />
          </TeamNarrativeNew>
          <TeamNarrativeReading>Reading 1 change</TeamNarrativeReading>
          <Button variant="link" onClick={none}>
            Catch me up
          </Button>
        </TeamNarrative>
        <TeamNarrative>
          <span>
            Only <strong>Rate limits</strong>: 2 changes.
          </span>
          <Button variant="link" onClick={none}>
            Show everything <Keys k="esc" />
          </Button>
        </TeamNarrative>
      </div>
    );
  },
};

export const Day: StoryObj = {
  render: () => (
    <div className="page">
      <TeamGroup label="Today">
        <GroupHead count={items.length}>Today</GroupHead>
        <Rows role="listbox" aria-label="Today">
          {items.map((it, i) => (
            <Fragment key={it.key}>
              {i === 2 && <TeamSeenLine at={at(2.5)} />}
              {row(it, { fresh: i < 2, selected: i === 1 })}
            </Fragment>
          ))}
        </Rows>
      </TeamGroup>
    </div>
  ),
};

// A change near more capsules of yours than are said one by one: one line
// between them.
export const NearMany: StoryObj = {
  render: () => (
    <div className="page">
      <Rows role="listbox" aria-label="Today">
        {row({ ...items[1], narrative: undefined, near: ["invoice-pdf", "vat-rates", "money-type", "pdf-fonts", "ledger"].map((c) => ({ capsule: `web/${c}`, files: ["web/src/invoices/pdf.ts"] })) })}
      </Rows>
    </div>
  ),
};

export const Opened: StoryObj = {
  render: () => (
    <div className="page">
      <TeamGroup label="Today">
        <GroupHead count={items.length}>Today</GroupHead>
        <Rows role="listbox" aria-label="Today">
          {items.map((it) => row(it, { open: true }))}
        </Rows>
      </TeamGroup>
    </div>
  ),
};

export const ByPerson: StoryObj = {
  render: () => (
    <div className="page">
      <TeamGroup label="Ana">
        <GroupHead title="ana">Ana</GroupHead>
        <Rows role="listbox" aria-label="Theirs">
          {row(items[0], { dated: true })}
          {row(items[3], { dated: true })}
        </Rows>
        <TeamSub>Reviewing</TeamSub>
        <Rows role="listbox" aria-label="Reviewing">
          {row(items[2], { dated: true })}
        </Rows>
      </TeamGroup>
      <TeamGroup label="James">
        <GroupHead title="james">James</GroupHead>
        <Rows role="listbox" aria-label="Theirs">
          {row(items[1], { dated: true })}
        </Rows>
      </TeamGroup>
    </div>
  ),
};
