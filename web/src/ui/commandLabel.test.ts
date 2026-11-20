import { describe, expect, it } from "vitest";
import type { CommandLabel } from "../wire.gen";
import { shellCommands, stepDoing, stepLabel } from "./commandLabel";

const table: CommandLabel[] = [
  { words: [], label: "Seeing workspace state", options: ["--format", "-f"] },
  { words: ["status"], label: "Seeing workspace state", options: ["--format", "-f"] },
  { words: ["show"], label: "Looking up {1}", options: ["--format", "-f"] },
  { words: ["mission", "show"], label: "Reading the mission", options: ["--format", "-f", "--mission", "-m"] },
  { words: ["mission", "note"], label: "Noting it in the log", options: ["-m"] },
  { words: ["silo", "point"], label: "Pointing {1} at {2}" },
  { words: ["sketch", "show"], label: "Pulling up {1}", options: ["--format", "-f"] },
  { words: ["sketch", "edit"], label: "Drawing on {1}", options: ["--format", "-f"] },
];

describe("stepLabel", () => {
  it("reads one of ws's commands past its flags", () => {
    expect(stepLabel("ws show -f llm sketch-5", table)).toBe("Looking up sketch-5");
    expect(stepLabel("ws status -f llm", table)).toBe("Seeing workspace state");
    expect(stepLabel("ws", table)).toBe("Seeing workspace state");
    expect(stepLabel("ws mission show -m m12 -f llm", table)).toBe("Reading the mission");
    expect(stepLabel("ws silo point demo fix-it", table)).toBe("Pointing demo at fix-it");
  });

  it("reads a plugin's command, and one with a heredoc", () => {
    expect(stepLabel("ws sketch show sketch-5 -f llm", table)).toBe("Pulling up sketch-5");
    expect(stepLabel(`ws sketch edit sketch-5 <<'EOF'\n[{"op":"add"}]\nEOF`, table)).toBe("Drawing on sketch-5");
  });

  it("joins ws's commands run together, past what's only alongside", () => {
    expect(stepLabel("ws sketch show sketch-5 -f llm; echo ---; ws status -f llm", table)).toBe("Pulling up sketch-5 · Seeing workspace state");
    expect(stepLabel("cd repos/demo && ws status -f llm | head -50", table)).toBe("Seeing workspace state");
    expect(stepLabel("WS_X=1 ~/.local/bin/workspace show t3", table)).toBe("Looking up t3");
  });

  it("leaves a step alone that runs anything else, or that ws can't read", () => {
    expect(stepLabel("cat README.md; echo ---; cat Makefile", table)).toBeUndefined();
    expect(stepLabel("ws status; git log", table)).toBeUndefined();
    expect(stepLabel("ws show", table)).toBeUndefined(); // no {1}
    expect(stepLabel("ws frobnicate x", table)).toBeUndefined();
    expect(stepLabel("echo hi", table)).toBeUndefined();
  });

  it("takes the words after -- as arguments", () => {
    expect(stepLabel("ws show -- -odd-ref", table)).toBe("Looking up -odd-ref");
  });
});

describe("shellCommands", () => {
  it("keeps what's quoted as one word, and skips a heredoc's body", () => {
    expect(shellCommands(`ws mission note "a; b" && x <<EOF\nnot | this\nEOF\ny`)).toEqual([["ws", "mission", "note", "a; b"], ["x"], ["y"]]);
  });
});

describe("stepDoing", () => {
  it("says what the agent's doing, ws's commands by their labels", () => {
    expect(stepDoing({ tool: "Bash", input: "ws show -f llm sketch-5" }, table)).toBe("Looking up sketch-5");
    expect(stepDoing({ tool: "Bash", input: "cd repos/demo/.ground && git ls-tree -r HEAD | head -200" }, table)).toBe("Running git");
    expect(stepDoing({ tool: "Read", input: "/w/repos/demo/main.go" }, table)).toBe("Reading main.go");
    expect(stepDoing({ tool: "Edit", input: "", edit: { path: "/w/x/README.md" } }, table)).toBe("Editing README.md");
    expect(stepDoing({ tool: "Grep", input: "func main" }, table)).toBe("Searching the code");
    expect(stepDoing({ tool: "TodoWrite", input: "", todos: [{ status: "in_progress", content: "Run tests", active: "Running tests" }] }, table)).toBe("Running tests");
  });
});
