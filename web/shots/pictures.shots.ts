// The README's and FEATURES.md's pictures: the web UI of a try seeded with
// scripts/seeds/marketing.sh, a view each. A picture is a page at the size
// the config says, as much of it as the screen shows.
//
// Add one here when a picture is worth taking more than once, and name it in
// an export in shotta.config.ts; what it shows goes in the seed, so the
// pictures stay of one workspace.
import { defineFlow } from "shotta";

const picture = (name: string, path: string, shows: string) => [{ goto: path }, { shot: name, viewport: true, shows }];

export default defineFlow({
  shows: "Each view of the web UI, of Sorrel's workspace on an ordinary day",
  // Nothing here is a journey to watch: each step opens a page and pictures it.
  video: false,
  steps: [
    // The README's, first: an export runs a flow only as far as the last shot it asks for.
    ...picture("mission", "/missions/invoice-totals-off-by-a-penny", "A mission's page: step 4 of 5 is yours, a review of the fix, with the plan, the crew and what happened beside it"),
    ...picture("missions", "/", "The missions list, in four groups: Needs you, In progress, Waiting and Up next"),
    { goto: "/missions/invoice-totals-off-by-a-penny/review/api/tax-rounding?view=changes" },
    // Its comment, in the middle of the page.
    { scroll: "text=Half away from zero" },
    { shot: "review", viewport: true, shows: "Reviewing a capsule: the diff of the fix, with a question in draft on one of its lines" },
    ...picture("verify", "/missions/export-invoices-as-csv?tab=verify&b=b1", "A verification bundle: two checks settled, and one left for you, shown before and after"),
    // The rest: FEATURES.md's.
    ...picture("brief", "/missions/invoice-totals-off-by-a-penny?tab=brief", "A mission's Brief tab: its sections and the plan's steps listed down the left, the brief and the plan as documents in the middle, and where the planner is asked on the right"),
    ...picture("work", "/missions/export-invoices-as-csv?tab=work", "A mission's Work tab: its capsules, and for the one that's open its crew, steps, commits and changed files"),
    ...picture("reviews", "/reviews", "Reviews: pull requests that need your review, one that's back with you, and your own"),
    ...picture("pull-request", "/reviews/api/41?view=changes", "Reviewing a teammate's pull request: the diff, with a thread from GitHub on one of its lines"),
    ...picture("documents", "/documents", "Documents: what changed lately, and the Records space with its kinds"),
    ...picture("space", "/documents/records", "The Records space: its documents by kind, with the one picked shown beside the list"),
    ...picture("document", "/documents/records/proposals/csv-export.md", "A proposal, read: its sections listed beside it, and a box to ask about it"),
    ...picture("artifact", "/artifacts/a1", "An artifact: a prototype of the invoices page, at its second version"),
    ...picture("conversation", "/conversations/w1", "A conversation: the agent's answer, and under it the mission it proposes, to create or decline"),
    ...picture("files", "/files/api?path=internal/invoice/invoice.go", "Files: the api's tree on its default branch, with a file open"),
    ...picture("team", "/team", "Team: what changed today, yesterday and before, a line each, with who did it"),
    ...picture("settings", "/settings/agents", "Settings, under Agents: each role's agent and model in one table, with whose each is"),
    ...picture("orchestrator", "/orchestrator", "The orchestrator's page: on, with its two latest runs and what each did"),
  ],
});
