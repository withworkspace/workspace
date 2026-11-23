import { useState } from "react";
import { Link } from "react-router";
import { useCommandLabels } from "../workspace/store";
import type { CommandLabel } from "../wire.gen";
import { stepLabel } from "./commandLabel";
import { Icon, type IconName } from "./Icon";
import { Spinner } from "./Spinner";
import { Chip } from "./Chip";
import { Button } from "./Button";
import { type Edit, EditDiff } from "./EditDiff";
import { type Todo, Todos } from "./Todos";

// Calls are the steps an agent took on the way to a reply: each tool it
// used, what it gave it, and (opened) what came back. Folded under "n
// steps", but for a run going now. The same in conversations, a crew's
// transcript and an orchestrator's run. Some steps read as themselves: an
// edit as a diff (with a link to the file), the plan as its items, a
// question as its options. A step that ran ws's commands reads as what
// it's doing ("Looking up sketch-5"), its command there when it's opened.

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

// CallsOptions: live is whether the agent's running (a pending step spins);
// root shortens paths; fileLink is where a file it edited can be read.
interface CallsOptions {
  root?: string;
  live?: boolean;
  fileLink?: (path: string) => string | undefined;
}

// tail: opened, only the last few steps show, the ones before them under
// a line that says how many (and shows them, asked): a run going now
// stays a few lines tall however long it goes on, so the page under it
// doesn't grow step by step and fall back when it's done.
export function Calls({ calls, open, tail, ...opts }: { calls: Call[]; open?: boolean; tail?: number } & CallsOptions) {
  const [all, setAll] = useState(false);
  const labels = useCommandLabels();
  if (!calls.length) return null;
  const refused = calls.filter((c) => c.refused).length;
  const failed = calls.filter((c) => c.error && !c.refused).length;
  const edits = calls.filter((c) => c.edit).length;
  // (Two more than tail before any are held back: a line saying "1
  // earlier step" would save nothing.)
  const from = open && tail && !all && calls.length > tail + 1 ? calls.length - tail : 0;
  const list = (
    <ol className="calls">
      {calls.map((c, i) => i >= from && <CallLine key={i} c={c} {...opts} labels={labels} running={opts.live && (c.pending ?? false)} />)}
    </ol>
  );
  if (open)
    return (
      <div className="calls-open">
        {from > 0 && (
          <Button variant="link" type="button" className="calls-earlier" onClick={() => setAll(true)}>
            <Icon name="chevron-right" size={12} /> {from} earlier steps
          </Button>
        )}
        {list}
      </div>
    );
  return (
    <details className="calls-fold">
      <summary>
        <Icon name="chevron-right" size={12} className="fold-chevron" />
        {calls.length} {calls.length === 1 ? "step" : "steps"}
        {edits > 0 && <Chip tone="quiet">{edits === 1 ? "1 edit" : `${edits} edits`}</Chip>}
        {refused > 0 && <Chip tone="danger">{refused} refused</Chip>}
        {failed > 0 && <Chip tone="quiet">{failed} failed</Chip>}
      </summary>
      {list}
    </details>
  );
}

function CallLine({ c, root, running, fileLink, labels }: { c: Call; running?: boolean; labels: readonly CommandLabel[] } & CallsOptions) {
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
    <li className={`call${c.refused ? " refused" : c.error ? " error" : ""}${open ? " open" : ""}${running ? " running" : ""}`}>
      <button className="call-line" onClick={() => body && setOpen((o) => !o)} aria-expanded={body ? open : undefined} disabled={!body}>
        {running ? <Spinner size={13} className="call-icon" /> : <Icon name={c.refused ? "ban" : iconFor(c.tool)} size={13} className="call-icon" />}
        <span className={`call-tool${doing ? " doing" : ""}`} title={doing ? short(c.input) : undefined}>
          {label}
        </span>
        {!doing && <span className="call-input">{what}</span>}
        {running && <Chip tone="progress">Running</Chip>}
        {c.refused && <Chip tone="danger">Refused</Chip>}
        {c.error && !c.refused && <Chip tone="quiet">Failed</Chip>}
      </button>
      {open && c.edit && (
        <div className="call-edit">
          <EditDiff edit={c.edit} />
          {link && (
            <Link to={link} className="call-file-link">
              <Icon name="file" size={12} /> Open {short(c.edit.path).split("/").pop()} in the capsule
            </Link>
          )}
        </div>
      )}
      {open && c.todos && <Todos todos={c.todos} className="call-todos" />}
      {open && c.questions?.map((q, i) => (
        <div key={i} className="call-question">
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
        <pre className="call-result">
          {doing && <div className="call-command">$ {short(c.input)}</div>}
          {result && short(result)}
        </pre>
      )}
    </li>
  );
}
