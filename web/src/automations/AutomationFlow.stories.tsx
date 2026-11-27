import type { Meta, StoryObj } from "@storybook/react-vite";
import type { AutomationOutline } from "../api";
import { AutomationFlow } from "./AutomationFlow";

// An automation's script, drawn: one that reads, branches, starts a
// conversation, waits for its checks with both ways a wait goes, and loops;
// with how each step's gone over its runs, as a dry run would say it, and
// one with no run of its own.

const meta: Meta<typeof AutomationFlow> = { title: "Automations/AutomationFlow", component: AutomationFlow };
export default meta;

const ciRed: AutomationOutline = {
  when: [{ match: "pr.checks_failed repo=api", pace: "at_most=1/1h per=pr" }],
  steps: [
    { kind: "read", at: "9:14", line: 9, text: "show(e.pr)", detail: "pr" },
    {
      kind: "if",
      at: "10:5",
      line: 10,
      text: "pr == None",
      branches: [
        { label: "if pr == None", steps: [{ kind: "stop", at: "11:9", line: 11 }] },
        { label: 'elif e.title.startswith("WIP")', steps: [{ kind: "log", at: "13:12", line: 13, text: "skipped a WIP" }] },
        { label: "else", steps: [{ kind: "converse", at: "15:17", line: 15, text: "Why did CI fail on {e.pr} ({e.title})?", detail: "about [e.pr]" }] },
      ],
    },
    {
      kind: "wait",
      at: "16:17",
      line: 16,
      text: "pr.checks_passed pr={e.pr}",
      detail: "up to 2h",
      branches: [
        { label: "met", steps: [{ kind: "notify", at: "6:11", line: 6, text: "{e.pr} is green again", detail: 'opens "api#1"' }] },
        { label: "time's up", steps: [{ kind: "notify", at: "18:15", line: 18, text: "{e.pr} is still red after 2h" }] },
      ],
    },
    { kind: "for", at: "21:5", line: 21, text: "c in e.commits", branches: [{ label: "for each c in e.commits", steps: [{ kind: "code", at: "22:9", line: 22, text: "n = len(c)" }] }] },
  ],
};

const steps = [
  { at: "9:14", builtin: "show", done: 9 },
  { at: "15:17", builtin: "converse", done: 6 },
  { at: "16:17", builtin: "wait", met: 4, timedOut: 1, waiting: 1 },
  { at: "6:11", builtin: "notify", done: 4 },
  { at: "18:15", builtin: "notify", done: 1, started: 1 },
];

export const WithRuns: StoryObj<typeof AutomationFlow> = { args: { outline: ciRed, steps, failures: { "9:14": 2 } } };

export const DryRun: StoryObj<typeof AutomationFlow> = { args: { outline: ciRed, steps, dry: true } };

export const NoCountsYet: StoryObj<typeof AutomationFlow> = { args: { outline: ciRed } };

export const NoRun: StoryObj<typeof AutomationFlow> = {
  args: { outline: { when: [{ match: "pr.merged repo=api" }, { match: "pr.closed repo=api" }], steps: [], default: true } },
};
