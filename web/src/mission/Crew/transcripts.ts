import type { Crew, Transcript, TranscriptEntry } from "../../api";
import { takenMissions } from "../../stories/missions";

// What the crew stories draw: a crew of a mission ws gave (the demo try's
// work-across-repos, c2, in its capsule), at each activity a crew's page
// shows, and transcripts of its session. No seed leaves crew with a
// transcript ws can read (a try's crew are stand-ins that keep no session
// file, and Claude Code keeps its own under the home folder, which a try
// shares with you), so the session is written here, in the form ws reads
// it as (harness.TranscriptEntry), as the Calls and Turn stories write
// theirs.

const mission = takenMissions.find((m) => m.from === "demo" && m.name === "work-across-repos")!.d;
export const slug = mission.slug;
const taken = mission.crew.find((c) => c.id === "c2")!;
const capsule = mission.capsules.find((c) => `${c.repo}/${c.name}` === taken.capsule)!;

// crewAt is the crew as Claude Code's, live at an activity, or ended.
export const crewAt = (activity?: Crew["activity"], more: Partial<Crew> = {}): Crew => ({
  ...taken,
  harness: "claude",
  model: "sonnet",
  activity,
  ...more,
});

const at = (minutes: number) => new Date(Date.parse(taken.started) + minutes * 60_000).toISOString();
const file = (rel: string) => `${capsule.path}/${rel}`;

const brief = `You are crew on the mission "Work across repos" (work-across-repos), as c2, in the capsule ${capsule.path}.

Step 3 of 5: Greet by name. greeting takes a name, and falls back to world.

When you're done, say so with ws mission done, and what you did.`;

const read: TranscriptEntry = { by: "call", tool: "Read", input: file("greet/greet.go"), result: "     1\tpackage greet\n     2\t\n     3\tfunc To() string { return \"hello, world\" }" };
const grep: TranscriptEntry = { by: "call", tool: "Grep", input: "greet.To", result: "main.go:9\ngreet/greet_test.go:6" };
const edit: TranscriptEntry = {
  by: "call",
  tool: "Edit",
  input: file("greet/greet.go"),
  result: "The file has been updated.",
  edit: { path: file("greet/greet.go"), old: 'func To() string { return "hello, world" }', new: 'func To(name string) string {\n\tif name == "" {\n\t\tname = "world"\n\t}\n\treturn "hello, " + name\n}' },
};
const plan = (now: number): TranscriptEntry => ({
  by: "call",
  tool: "TodoWrite",
  input: "",
  result: "Todos have been modified.",
  todos: [
    { content: "Give greet.To a name", status: now > 0 ? "completed" : "in_progress", active: "Giving greet.To a name" },
    { content: "Fall back to world", status: now > 1 ? "completed" : now === 1 ? "in_progress" : "pending", active: "Falling back to world" },
    { content: "Test it", status: now > 2 ? "completed" : now === 2 ? "in_progress" : "pending", active: "Testing it" },
    { content: "Say how in the README", status: now > 3 ? "completed" : now === 3 ? "in_progress" : "pending", active: "Saying how in the README" },
  ],
});
const test: TranscriptEntry = { by: "call", tool: "Bash", input: "go test ./...", result: "ok  \tdemo/greet\t0.214s" };
const failed: TranscriptEntry = { by: "call", tool: "Bash", input: "go vet ./...", result: "main.go:9:21: not enough arguments in call to greet.To", error: true };

// A session that's gone on a while: its brief, what it was told, what it
// said, its steps, an interruption, and the latest of its plan.
export const session: Transcript = {
  harness: "claude",
  available: true,
  version: 4096,
  entries: [
    { by: "user", text: brief, at: at(0) },
    { by: "agent", text: "I'll start by reading **greet.To** and what calls it.", at: at(1) },
    read,
    grep,
    plan(0),
    { by: "agent", text: "`greet.To` takes nothing yet. I'll give it a name, and fall back to `world` when it's empty.", at: at(2) },
    edit,
    failed,
    { by: "user", text: "[Request interrupted by user]" },
    { by: "user", text: "Keep main.go's call as it is: pass it the flag's value.", at: at(4) },
    { by: "agent", text: "Done: `main.go` passes `*name`, and the tests pass.\n\n- `greet.To(\"\")` says hello, world\n- `greet.To(\"Ada\")` says hello, Ada", at: at(6) },
    plan(3),
    test,
  ],
};

// The same, its latest entries only: those before are left out.
export const latest: Transcript = { ...session, earlier: 12, entries: session.entries.slice(5) };

// The same, at work: its last step not back yet.
export const working = (step: TranscriptEntry): Transcript => ({ ...session, entries: [...session.entries.slice(0, 7), { ...step, result: undefined, pending: true }] });

export const runningTests = working(test);
export const editing = working(edit);
export const toRun = working({ by: "call", tool: "Bash", input: "rm -rf build && go build -o build/greet ./..." });
export const asking: Transcript = working({ by: "call", tool: "AskUserQuestion", input: "", questions: [{ question: "Should greet.To trim the name it's given?", options: ["Trim it", "Leave it as it's given"] }] });

export const none: Transcript = { harness: "shell", available: false, version: 0, entries: [], why: "shell keeps no readable transcript: see the terminal." };
export const nothingYet: Transcript = { harness: "claude", available: true, version: 0, entries: [] };
export const root = "/try/demo";
