import type { CommandLabel } from "../wire.gen";
import { clip } from "./strings";

// A step that ran one of ws's commands reads as what it's doing: "Looking
// up sketch-5" for `ws show sketch-5 -f llm`. ws says each command's
// words (GET /api/commands); this reads the script the agent ran.

// shellCommands is a script's commands, each as its words, read loosely
// (as internal/harness's looseCommands does): what's quoted is one word,
// a heredoc's body and where a redirection goes are neither.
export function shellCommands(s: string): string[][] {
  const out: string[][] = [];
  let words: string[] = [];
  let pending: { delim: string; tabbed: boolean }[] = [];
  const end = () => {
    if (words.length) out.push(words);
    words = [];
  };
  let i = 0;
  while (i < s.length) {
    const c = s[i];
    if (c === " " || c === "\t" || c === "\r") {
      i++;
    } else if (c === "\\" && s[i + 1] === "\n") {
      i += 2;
    } else if (c === "\n") {
      i++;
      end();
      for (const h of pending) {
        while (i < s.length) {
          const stop = s.indexOf("\n", i);
          let line = stop < 0 ? s.slice(i) : s.slice(i, stop);
          i = stop < 0 ? s.length : stop + 1;
          if (h.tabbed) line = line.replace(/^\t+/, "");
          if (line === h.delim) break;
        }
      }
      pending = [];
    } else if (s.startsWith("<<", i) && !s.startsWith("<<<", i)) {
      i += 2;
      const tabbed = s[i] === "-";
      if (tabbed) i++;
      while (s[i] === " " || s[i] === "\t") i++;
      const [delim, next] = shellWord(s, i);
      i = next;
      if (delim) pending.push({ delim, tabbed });
    } else if (c === "<" || c === ">") {
      while (i < s.length && "<>&|".includes(s[i])) i++;
      while (s[i] === " " || s[i] === "\t") i++;
      i = shellWord(s, i)[1];
    } else if (";|&(){}`".includes(c)) {
      i++;
      end();
    } else if (c === "$" && s[i + 1] === "(") {
      i += 2;
      end();
    } else if (c === "#" && words.length === 0) {
      while (i < s.length && s[i] !== "\n") i++;
    } else {
      const [word, next] = shellWord(s, i);
      i = next === i ? i + 1 : next;
      if (word) words.push(word);
    }
  }
  end();
  return out;
}

// shellWord is the word starting at i, its quotes off, and where the
// script goes on after it.
function shellWord(s: string, i: number): [string, number] {
  let w = "";
  while (i < s.length) {
    const c = s[i];
    if (c === "'") {
      const j = s.indexOf("'", i + 1);
      if (j < 0) return [w + s.slice(i + 1), s.length];
      w += s.slice(i + 1, j);
      i = j + 1;
    } else if (c === '"') {
      i++;
      while (i < s.length && s[i] !== '"') {
        if (s[i] === "\\" && i + 1 < s.length) i++;
        w += s[i];
        i++;
      }
      i++;
    } else if (c === "\\" && i + 1 < s.length) {
      w += s[i + 1];
      i += 2;
    } else if (" \t\r\n;|&(){}<>`".includes(c) || (c === "$" && s[i + 1] === "(")) {
      return [w, i];
    } else {
      w += c;
      i++;
    }
  }
  return [w, i];
}

// Words before a command that only say how it's run.
const wrappers = new Set(["time", "env", "command", "exec", "nohup", "builtin"]);

// Commands that change nothing of what a step reads as, run alongside
// ws's: a separator echoed, a directory gone into, output cut short.
const asides = new Set(["echo", "printf", "true", ":", "cd", "head", "tail", "sleep"]);

const isWS = (word: string) => /(^|\/)(ws|workspace)$/.test(word);

// stepLabel is what a script the agent ran is doing, where every command
// in it is one of ws's with a label (or an aside to one): undefined for
// one that runs anything else, so the step shows its script.
export function stepLabel(script: string, table: readonly CommandLabel[]): string | undefined {
  const byWords = new Map(table.map((l) => [l.words.join(" "), l]));
  const said: string[] = [];
  for (let words of shellCommands(script)) {
    while (words.length && (/^[A-Za-z_][A-Za-z0-9_]*=/.test(words[0]) || wrappers.has(words[0]))) words = words.slice(1);
    if (!words.length) continue;
    if (asides.has(words[0])) continue;
    if (!isWS(words[0])) return undefined;
    const label = labelOf(words.slice(1), byWords);
    if (!label) return undefined;
    if (said[said.length - 1] !== label) said.push(label);
  }
  return said.length ? said.join(" · ") : undefined;
}

// labelOf is one ws command's label with its arguments in, from the
// words after ws: the longest run of words it starts with that names a
// command, then what's left read past its flags.
function labelOf(words: string[], byWords: Map<string, CommandLabel>): string | undefined {
  let named = 0;
  while (named < words.length && !words[named].startsWith("-")) named++;
  // ws on its own is ws status; ws and a word it doesn't know is no command.
  for (let n = named; n >= (named ? 1 : 0); n--) {
    const l = byWords.get(words.slice(0, n).join(" "));
    if (l) return fill(l.label, args(words.slice(n), l.options ?? []));
  }
  return undefined;
}

// args are a command's arguments, past its flags and their values.
function args(words: string[], options: readonly string[]): string[] {
  const out: string[] = [];
  for (let i = 0; i < words.length; i++) {
    const w = words[i];
    if (w === "--") return out.concat(words.slice(i + 1));
    if (w === "--stdin-from" || options.includes(w)) i++;
    else if (w.length > 1 && w.startsWith("-")) continue;
    else out.push(w);
  }
  return out;
}

// fill puts the arguments in a label's {1}, {2}: undefined where it
// names one that isn't there. A long one is cut short.
function fill(label: string, args: string[]): string | undefined {
  let missing = false;
  const out = label.replace(/\{(\d+)\}/g, (_, n: string) => {
    const a = args[Number(n) - 1];
    if (a === undefined || a === "") {
      missing = true;
      return "";
    }
    const one = a.replace(/\s+/g, " ").trim();
    return clip(one, 48);
  });
  return missing ? undefined : out;
}

// A step as what the agent's doing, in a few words, for a run going now:
// ws's commands as their labels, the rest as the tool says ("Reading
// main.go", "Running git"). Only what's needed to see it's moving: the
// steps themselves are a click away.
export interface Step {
  tool: string;
  input: string;
  edit?: { path: string };
  todos?: { status: string; content: string; active?: string }[];
  questions?: unknown[];
}

const base = (p: string) => p.replace(/\/+$/, "").split("/").pop() || p;

export function stepDoing(c: Step, table: readonly CommandLabel[]): string {
  const tool = c.tool.toLowerCase();
  if (c.todos) {
    const now = c.todos.find((t) => t.status === "in_progress");
    return now ? (now.active ?? now.content) : "Planning";
  }
  if (c.questions?.length || tool === "askuserquestion") return "Asking you";
  if (c.edit) return `${tool.startsWith("write") ? "Writing" : "Editing"} ${base(c.edit.path)}`;
  if (/^(bash|shell|exec|terminal|run)/.test(tool)) {
    const label = stepLabel(c.input, table);
    if (label) return label;
    for (let words of shellCommands(c.input)) {
      while (words.length && (/^[A-Za-z_][A-Za-z0-9_]*=/.test(words[0]) || wrappers.has(words[0]))) words = words.slice(1);
      if (words.length && !asides.has(words[0])) return `Running ${base(words[0])}`;
    }
    return "Running a command";
  }
  if (/^(read|view|cat)/.test(tool)) return c.input ? `Reading ${base(c.input)}` : "Reading";
  if (/^(grep|glob|search|find|ls)/.test(tool)) return "Searching the code";
  if (tool === "websearch") return "Searching the web";
  if (/^(webfetch|fetch)/.test(tool)) return "Reading the web";
  if (/^(task|agent)/.test(tool)) return "Handing it to an agent";
  return c.tool;
}
