import type { Meta, StoryObj } from "@storybook/react-vite";
import { type Call, Calls } from "./Calls";

// An agent's steps: folded under their count, with what's worth knowing
// before they're opened (edits, a refusal, a failure); open, as a run's
// are; a run going now, its last step turning, cut to its last few; and
// each kind of step, to press: what came back, an edit as a diff, the
// todo list, a question.

const meta: Meta = { title: "Components/Calls" };
export default meta;

const root = "/Users/sam/sorrel";

const read: Call = { tool: "Read", input: `${root}/api/invoice.go`, result: "     1\tpackage api\n     2\t\n     3\tfunc roundDiv(a, b int64) int64 {" };
const grep: Call = { tool: "Grep", input: "roundDiv", result: "api/invoice.go:3\napi/dashboard.go:41" };
const edit: Call = {
  tool: "Edit",
  input: `${root}/api/dashboard.go`,
  edit: { path: `${root}/api/dashboard.go`, old: "\treturn total / n", new: "\t// Half away from zero, as invoice.go rounds.\n\treturn roundDiv(total, n)" },
};
const todos: Call = {
  tool: "TodoWrite",
  input: "",
  todos: [
    { content: "Find where amounts are divided", status: "completed" },
    { content: "Give the dashboard roundDiv", status: "in_progress", active: "Giving the dashboard roundDiv" },
    { content: "Run the tests", status: "pending" },
  ],
};
const asked: Call = { tool: "AskUserQuestion", input: "", questions: [{ question: "Round the old invoices again too?", options: ["Yes, all of them", "Only those not yet sent"] }] };
const failed: Call = { tool: "Bash", input: "go test ./api/...", result: "--- FAIL: TestShare (0.00s)\n    dashboard_test.go:18: got 33, want 34", error: true };
const refused: Call = { tool: "Bash", input: "git push --force origin main", result: "Not allowed here.", refused: true, error: true };
const quiet: Call = { tool: "Bash", input: "true" };
const long: Call = { tool: "Bash", input: `cd ${root} && go vet ./... && go test ./api/... -run 'TestShare|TestRound' -count=1 -v | tail -40`, result: "ok  \tsorrel/api\t0.412s" };

const all = [read, grep, edit, todos, asked, failed, refused, quiet, long];

export const Folded: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, maxWidth: 560, justifyItems: "start" }}>
      <Calls calls={[read]} root={root} />
      <Calls calls={[read, grep]} root={root} />
      <Calls calls={all} root={root} fileLink={(p) => `/files${p}`} />
    </div>
  ),
};

export const Open: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 560 }}>
      <Calls calls={all} root={root} open fileLink={(p) => `/files${p}`} />
    </div>
  ),
};

export const Running: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24, maxWidth: 560 }}>
      <Calls calls={[read, grep, { tool: "Bash", input: "go test ./api/...", pending: true }]} root={root} open live />
      <div>
        <div className="dim">Its last three, the rest asked for</div>
        <Calls calls={[...all, { tool: "Bash", input: "go test ./api/...", pending: true }]} root={root} open live tail={3} />
      </div>
    </div>
  ),
};
