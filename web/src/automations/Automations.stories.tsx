import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { MemoryRouter } from "react-router";
import type { AutomationDryRun, AutomationItem, AutomationRun } from "../api";
import { GroupHead } from "../ui/GroupHead";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { AutomationFacts } from "./AutomationFacts";
import { AutomationRow } from "./AutomationRow";
import { AutomationRunItem } from "./AutomationRunItem";
import { AutomationRuns } from "./AutomationRuns";
import { AutomationScript } from "./AutomationScript";
import { AutomationsFrame, AutomationsLede, AutomationsSection } from "./AutomationsFrame";
import { DryRunShown } from "./DryRunShown";

// The Automations pages, from their parts: the list (on, off, failing,
// one whose script can't be read, one that hasn't run), an automation's
// runs (each state, what each did), its dry run, and its script.

const meta: Meta = {
  title: "Automations/Automations",
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};
export default meta;

const ago = (m: number) => new Date(Date.now() - m * 60_000).toISOString();

const drawing: AutomationItem = {
  name: "drawing",
  about: "Notify me if Brendan pushes a new drawing.",
  when: 'thing.added kind=sketch by=brendan',
  may: ["notify"],
  on: true,
  added: ago(60 * 24 * 3),
  tally: { runs: 4 },
  last: { n: 4, event: "e15", says: "Brendan added Release flow", at: ago(12), state: "ran", did: [{ what: "notify", text: "Brendan drew Release flow" }] },
};

const list: AutomationItem[] = [
  drawing,
  {
    name: "ana-sketches",
    about: "Tell me when Ana's done changing a sketch.",
    when: "thing.changed kind=sketch by=ana (settle=1m per=thing)",
    may: ["notify"],
    on: true,
    added: ago(30),
    tally: { runs: 0, settling: 1 },
  },
  {
    name: "ci-red",
    about: "When CI fails on main, find out why.",
    when: "pr.checks_failed repo=api",
    may: ["converse"],
    on: true,
    added: ago(60 * 24 * 10),
    failed: "ci-red.star:6:9: show: got int, want string",
    tally: { runs: 9, failed: 2, held: 3 },
  },
  {
    name: "landings",
    about: "Every landing on main, for the release notes.",
    when: "commits.landed repo=api",
    may: [],
    on: false,
    added: ago(60 * 24 * 20),
    tally: { runs: 31 },
    last: { n: 31, event: "e9", says: "Ana landed 3 commits on api", at: ago(60 * 24 * 2), state: "ran" },
  },
  { name: "half-written", may: [], on: false, added: ago(5), broken: "half-written.star:3:1: got newline, want primary expression", tally: { runs: 0 } },
];

function List() {
  const [items, setItems] = useState(list);
  return (
    <Rows>
      {items.map((a) => (
        <AutomationRow key={a.name} a={a} onOn={(on) => setItems((was) => was.map((b) => (b.name === a.name ? { ...b, on } : b)))} />
      ))}
    </Rows>
  );
}

export const Page: StoryObj = {
  render: () => (
    <AutomationsFrame>
      <PageHead title="Automations" />
      <AutomationsLede>What ws does by itself when something happens: tell you, or start a conversation to look into it.</AutomationsLede>
      <List />
    </AutomationsFrame>
  ),
};

const runs: AutomationRun[] = [
  { n: 12, event: "e52", says: "Checks failed on api#215 Cache keys", at: ago(1), state: "waiting", key: "api#215", did: [{ what: "ask", waiting: true, text: "for an agent's answer" }] },
  {
    n: 11,
    event: "e49",
    says: "Checks failed on api#214 Queue retries",
    at: ago(3),
    state: "waiting",
    key: "api#214",
    did: [
      { what: "ask", text: "flaky: false · why: TestRetry asserts on wall-clock time" },
      { what: "propose", text: "a mission: Fix TestRetry's clock", conversation: "w15" },
      { what: "wait", waiting: true, text: "for your answer to w15#1, till Nov 30 08:40" },
    ],
  },
  { n: 9, event: "e41", says: "Checks failed on api#212 Retry flaky uploads", at: ago(4), state: "waiting", key: "api#212", did: [{ what: "wait", waiting: true, text: "for pr.checks_passed pr=api#212, till Nov 27 09:12" }] },
  { n: 8, event: "e38", says: "Checks failed on api#209 Bump Go", at: ago(90), state: "ran", key: "api#209", did: [{ what: "converse", conversation: "w12" }], log: ["looked at 3 failing jobs"] },
  { n: 7, event: "e33", says: "Checks failed on api#209 Bump Go", at: ago(95), state: "held", key: "api#209", error: "held back: it's run 1 of at most 1 in 1h for api#209" },
  { n: 10, event: "e44", says: "triage answered in w14", at: ago(2), state: "held", error: "would loop: ci-red → triage → ci-red" },
  { n: 6, event: "e30", says: "Checks failed on api#204 New uploads", at: ago(60 * 5), state: "failed", error: "ci-red.star:6:9: show: got int, want string" },
  { n: 5, event: "e22", says: "Checks failed on api#201 Docs", at: ago(60 * 26), state: "unsure", did: [{ what: "notify", text: "api#201's checks are red", started: true }] },
  { n: 4, event: "e15", says: "Brendan added Release flow", at: ago(60 * 50), state: "ran", did: [{ what: "notify", text: "Brendan drew Release flow" }, { what: "wait", text: "e17: Ana changed Release flow" }] },
];

export const Runs: StoryObj = {
  render: () => (
    <AutomationsFrame>
      <AutomationsSection>
        <GroupHead count="9 runs · 2 failed · 1 waiting · 3 held back">Runs</GroupHead>
        <AutomationRuns>
          {runs.map((r) => (
            <AutomationRunItem key={r.n} r={r} />
          ))}
        </AutomationRuns>
      </AutomationsSection>
    </AutomationsFrame>
  ),
};

const dry: AutomationDryRun = {
  since: ago(60 * 24 * 30),
  events: 15,
  matched: 3,
  ran: 2,
  settling: 1,
  lines: [
    { event: "e4", at: ago(60 * 24 * 9), says: "Ana changed Release flow", batch: 2, did: ["notify: Ana saved Release flow 2 times"] },
    { event: "e9", at: ago(60 * 24 * 2), says: "Ana changed Onboarding", did: ["notify: Ana saved Onboarding"] },
    { event: "e15", at: ago(1), says: "Ana changed Release flow", waited: ["settling still: it runs once these have been quiet 1m"] },
  ],
};

export const Detail: StoryObj = {
  render: () => (
    <AutomationsFrame>
      <PageHead title="drawing" />
      <p>{drawing.about}</p>
      <AutomationFacts when={drawing.when} may={drawing.may} />
      <AutomationsSection>
        <GroupHead>Dry run</GroupHead>
        <DryRunShown dr={dry} />
      </AutomationsSection>
      <AutomationScript
        open
        source={`# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
`}
      />
    </AutomationsFrame>
  ),
};
