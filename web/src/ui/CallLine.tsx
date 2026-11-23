import { useState } from "react";
import { Link } from "react-router";
import type { CommandLabel } from "../wire.gen";
import css from "./CallLine.module.css";
import { Chip } from "./Chip";
import { stepLabel } from "./commandLabel";
import { cx } from "./cx";
import { type Edit, EditDiff } from "./EditDiff";
import { Icon, type IconName } from "./Icon";
import { Spinner } from "./Spinner";
import { type Todo, Todos } from "./Todos";

// Call is one step an agent took: the tool it used, what it gave it, and
// what came back. An edit, a todo list and a question are there as what
// they are, to be drawn as themselves.
export interface Call {
  tool: string;
  input: string;
  result?: string;
  error?: boolean;
  refused?: boolean;
  pending?: boolean;
  edit?: Edit;
  todos?: Todo[];
  questions?: { question: string; options?: string[] }[];
}

const toolIcons: [RegExp, IconName][] = [
  [/^(read|view|cat)/i, "book"],
  [/^(bash|shell|exec|run|terminal)/i, "terminal"],
  [/^(grep|glob|search|find|ls)/i, "search"],
  [/^(edit|write|multiedit|notebookedit|apply)/i, "pencil"],
  [/^(webfetch|websearch|fetch)/i, "external"],
  [/^(task|agent)/i, "sparkle"],
  [/^(todo)/i, "check-circle"],
  [/^askuserquestion/i, "question"],
];

const iconFor = (tool: string): IconName => toolIcons.find(([re]) => re.test(tool))?.[1] ?? "tool";

export interface CallLineProps {
  c: Call;
  // It's going now: its icon turns.
  running?: boolean;
  // The workspace's folder: paths in it read from there.
  root?: string;
  // Where a file it edited can be read.
  fileLink?: (path: string) => string | undefined;
  // What ws's commands read as (ui/commandLabel.ts).
  labels: readonly CommandLabel[];
}

// CallLine is one step, as a line of a list of them (Calls.tsx): its
// tool's icon, the tool, and what it was given, cut to the line. Pressed,
// it opens on what came back. Some read as themselves: an edit as a diff
// (with a link to the file), the todo list as its items, a question as
// its options; and a step that ran ws's commands reads as what it's doing
// ("Looking up sketch-5"), its command there when it's opened.
export function CallLine({ c, root, running, fileLink, labels }: CallLineProps) {
  const [open, setOpen] = useState(false);
  // Paths in the workspace read from its root.
  const short = (s: string) => (root ? s.split(root + "/").join("") : s);
  // A file read comes back with its line numbers first ("  12\t…"):
  // dropped, they only get in the way.
  const raw = c.result?.trim();
  const result = raw && /^(read|view)/i.test(c.tool) ? raw.replace(/^\s*\d+[\t→]/gm, "") : raw;
  const link = c.edit && fileLink?.(c.edit.path);
  const what = c.todos
    ? `${c.todos.filter((t) => t.status === "completed").length} of ${c.todos.length} done`
    : c.questions?.length
      ? c.questions.map((q) => q.question).join(" · ")
      : c.edit
        ? short(c.edit.path)
        : short(c.input);
  const doing = /^(bash|shell|exec|terminal)/i.test(c.tool) ? stepLabel(c.input, labels) : undefined;
  const label = c.todos ? "Plan" : c.questions?.length ? "Asked" : (doing ?? c.tool);
  const body = c.edit || c.todos || c.questions?.length || result || doing;
  return (
    <li className={cx(css.call, c.refused ? css.refused : c.error && css.failed, running && css.running)}>
      <button className={css.line} data-ui="" onClick={() => body && setOpen((o) => !o)} aria-expanded={body ? open : undefined} disabled={!body}>
        {running ? <Spinner size={13} className={css.icon} /> : <Icon name={c.refused ? "ban" : iconFor(c.tool)} size={13} className={css.icon} />}
        <span className={cx(css.tool, doing && css.doing)} title={doing ? short(c.input) : undefined}>
          {label}
        </span>
        {!doing && <span className={css.input}>{what}</span>}
        {running && <Chip tone="progress">Running</Chip>}
        {c.refused && <Chip tone="danger">Refused</Chip>}
        {c.error && !c.refused && <Chip tone="quiet">Failed</Chip>}
      </button>
      {open && c.edit && (
        <div className={css.edit}>
          <EditDiff edit={c.edit} />
          {link && (
            <Link to={link} className={css.fileLink}>
              <Icon name="file" size={12} /> Open {short(c.edit.path).split("/").pop()} in the capsule
            </Link>
          )}
        </div>
      )}
      {open && c.todos && <Todos todos={c.todos} className={css.todos} />}
      {open &&
        c.questions?.map((q, i) => (
          <div key={i} className={css.question}>
            <div>{q.question}</div>
            {q.options && (
              <ul>
                {q.options.map((o) => (
                  <li key={o}>{o}</li>
                ))}
              </ul>
            )}
          </div>
        ))}
      {open && !c.edit && !c.todos && !c.questions?.length && (result || doing) && (
        <pre className={css.result}>
          {doing && <div className={css.command}>$ {short(c.input)}</div>}
          {result && short(result)}
        </pre>
      )}
    </li>
  );
}
