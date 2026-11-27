import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ProposedAutomation } from "../api";
import { AutomationProposed } from "./AutomationProposed";

// A proposed automation, on its card: a rule that would have notified
// twice; a workflow whose waits were met, timed out and still waiting;
// paced (held back, settled); one that would have failed; and one nothing
// was for yet.

const meta: Meta<typeof AutomationProposed> = { title: "Talk/AutomationProposed", component: AutomationProposed };
export default meta;
type Story = StoryObj<typeof AutomationProposed>;

const drawing = `# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
`;

const base = (a: Partial<ProposedAutomation>): ProposedAutomation => ({
  name: "drawing",
  about: "Notify me if Brendan pushes a new drawing.",
  when: "thing.added kind=sketch by=brendan",
  may: ["notify"],
  dryRun: { since: "2026-10-27T10:00:00Z", events: 412, matched: 2, ran: 2, lines: [] },
  ...a,
});

export const Rule: Story = {
  args: {
    script: drawing,
    a: base({
      dryRun: {
        since: "2026-10-27T10:00:00Z", events: 412, matched: 2, ran: 2,
        lines: [
          { event: "e881", at: "2026-11-12T09:14:00Z", says: "Draw the release flow", did: ["notify: Brendan drew Release flow"] },
          { event: "e1203", at: "2026-11-24T16:02:00Z", says: "Queues for each tenant", did: ["notify: Brendan drew Queues"] },
        ],
      },
    }),
  },
};

export const Workflow: Story = {
  args: {
    script: "# After my PR merges, wait for the deploy, then move its issue to Done.\n…",
    a: base({
      name: "deploy-done",
      about: "After my PR merges, wait for the deploy to pass, then tell me.",
      when: "pr.merged by=me",
      dryRun: {
        since: "2026-10-27T10:00:00Z", events: 412, matched: 3, ran: 3, waiting: 1,
        lines: [
          { event: "e1150", at: "2026-11-20T11:00:00Z", says: "api#205 merged", waited: ["waited 14m for e1153 (passed)"], did: ["notify: deployed api#205"] },
          { event: "e1188", at: "2026-11-22T15:30:00Z", says: "web#97 merged", waited: ["waited 2h for run.ended run=deploy, and its time was up"], did: ["notify: web#97 merged but its deploy didn't pass"] },
          { event: "e1290", at: "2026-11-26T18:40:00Z", says: "api#212 merged", waited: ["would be waiting still, for run.ended run=deploy"] },
        ],
      },
    }),
  },
};

export const Paced: Story = {
  args: {
    script: "…",
    a: base({
      name: "red",
      about: "Tell me when my PR goes red, at most once an hour.",
      when: "pr.checks_failed by=me (at_most=1/1h per=pr)",
      may: [],
      dryRun: {
        since: "2026-10-27T10:00:00Z", events: 412, matched: 4, ran: 2, held: 2,
        lines: [
          { event: "e901", at: "2026-11-13T10:00:00Z", says: "api#1: Red", did: ["notify: api#1: Red"] },
          { event: "e902", at: "2026-11-13T10:10:00Z", says: "api#1: Red", held: "it'd have run 1 of at most 1 in 1h for api#1" },
          { event: "e905", at: "2026-11-13T10:20:00Z", says: "api#1: Red", held: "it'd have run 1 of at most 1 in 1h for api#1" },
          { event: "e930", at: "2026-11-14T09:00:00Z", says: "Flow", batch: 4, did: ["notify: 4 saves"] },
        ],
      },
    }),
  },
};

export const Failing: Story = {
  args: {
    script: "…",
    a: base({
      dryRun: {
        since: "2026-10-27T10:00:00Z", events: 412, matched: 1, ran: 1, failed: 1,
        lines: [{ event: "e881", at: "2026-11-12T09:14:00Z", says: "Draw the release flow", error: "drawing.star:6:12: struct has no .nope attribute" }],
      },
    }),
  },
};

export const NothingYet: Story = {
  args: { script: drawing, a: base({ replaces: true, dryRun: { since: "2026-10-27T10:00:00Z", events: 412, matched: 0, ran: 0, lines: [] } }) },
};

// One whose conversations would set off one you have, and that one of
// yours sets off: where a chain could start.
export const SetsOthersOff: Story = {
  args: {
    script: drawing,
    a: base({ setsOff: ["triage"], setOffBy: ["ci-red"], dryRun: { since: "2026-10-27T10:00:00Z", events: 412, matched: 3, ran: 2, held: 1, lines: [{ event: "e40", at: "2026-11-20T10:00:00Z", says: "triage answered in w14", held: "it'd loop: ci-red → triage → ci-red" }] } }),
  },
};
