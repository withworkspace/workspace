import { useState } from "react";
import { Link } from "react-router";
import { Icon, type IconName } from "./Icon";

// Calls are the steps an agent took on the way to a reply: each tool it
// used, what it gave it, and (opened) what came back. Folded under "n
// steps", but for a run going now. The same in conversations, a crew's
// transcript and an orchestrator's run. Some steps read as themselves: an
// edit as a diff (with a link to the file), the plan as its items, a
// question as its options.

export interface Call {
  tool: string;
  input: string;
  result?: string;
  error?: boolean;
  refused?: boolean;
  pending?: boolean;
  edit?: { path: string; old?: string; new: string; all?: boolean };
  todos?: { content: string; status: string; active?: string }[];
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
  if (!calls.length) return null;
  const refused = calls.filter((c) => c.refused).length;
  const failed = calls.filter((c) => c.error && !c.refused).length;
  const edits = calls.filter((c) => c.edit).length;
  // (Two more than tail before any are held back: a line saying "1
  // earlier step" would save nothing.)
  const from = open && tail && !all && calls.length > tail + 1 ? calls.length - tail : 0;
  const list = (
    <ol className="calls">
      {calls.map((c, i) => i >= from && <CallLine key={i} c={c} {...opts} running={opts.live && (c.pending ?? false)} />)}
    </ol>
  );
  if (open)
    return (
      <div className="calls-open">
        {from > 0 && (
          <button type="button" className="link calls-earlier" onClick={() => setAll(true)}>
            <Icon name="chevron-right" size={12} /> {from} earlier steps
          </button>
        )}
        {list}
      </div>
    );
  return (
    <details className="calls-fold">
      <summary>
        <Icon name="chevron-right" size={12} className="fold-chevron" />
        {calls.length} {calls.length === 1 ? "step" : "steps"}
        {edits > 0 && <span className="chip tone-quiet">{edits === 1 ? "1 edit" : `${edits} edits`}</span>}
        {refused > 0 && <span className="chip tone-danger">{refused} refused</span>}
        {failed > 0 && <span className="chip tone-quiet">{failed} failed</span>}
      </summary>
      {list}
    </details>
  );
}

function CallLine({ c, root, running, fileLink }: { c: Call; running?: boolean } & CallsOptions) {
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
  const label = c.todos ? "Plan" : c.questions?.length ? "Asked" : c.tool;
  const body = c.edit || c.todos || c.questions?.length || result;
  return (
    <li className={`call${c.refused ? " refused" : c.error ? " error" : ""}${open ? " open" : ""}${running ? " running" : ""}`}>
      <button className="call-line" onClick={() => body && setOpen((o) => !o)} aria-expanded={body ? open : undefined} disabled={!body}>
        <Icon name={running ? "spinner" : c.refused ? "ban" : iconFor(c.tool)} size={13} className={`call-icon${running ? " spin" : ""}`} />
        <span className="call-tool">{label}</span>
        <span className="call-input">{what}</span>
        {running && <span className="chip tone-progress">Running</span>}
        {c.refused && <span className="chip tone-danger">Refused</span>}
        {c.error && !c.refused && <span className="chip tone-quiet">Failed</span>}
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
      {open && c.todos && <Plan todos={c.todos} />}
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
      {open && !c.edit && !c.todos && !c.questions?.length && result && <pre className="call-result">{short(result)}</pre>}
    </li>
  );
}

// EditDiff is an edit as a diff: what it replaced, and with what (a file
// written whole is all new). Long ones show their start.
export function EditDiff({ edit, max = 14 }: { edit: NonNullable<Call["edit"]>; max?: number }) {
  const [all, setAll] = useState(false);
  const old = edit.old ? edit.old.split("\n") : [];
  const next = edit.new.split("\n");
  const total = old.length + next.length;
  const cut = (lines: string[], room: number) => (all ? lines : lines.slice(0, Math.max(room, 0)));
  const oldShown = cut(old, Math.ceil(max / 2));
  const newShown = cut(next, max - oldShown.length);
  return (
    <div className="edit-diff">
      {oldShown.map((l, i) => (
        <div key={`o${i}`} className="sl d">
          <span className="sign">−</span>
          {l || " "}
        </div>
      ))}
      {newShown.map((l, i) => (
        <div key={`n${i}`} className="sl a">
          <span className="sign">+</span>
          {l || " "}
        </div>
      ))}
      {!all && total > oldShown.length + newShown.length && (
        <button className="link edit-more" onClick={() => setAll(true)}>
          {total - oldShown.length - newShown.length} more lines
        </button>
      )}
    </div>
  );
}

// Plan is an agent's todo list: done, in hand, to do.
export function Plan({ todos, live }: { todos: NonNullable<Call["todos"]>; live?: boolean }) {
  return (
    <ul className="plan-items">
      {todos.map((t, i) => (
        <li key={i} className={`plan-item s-${t.status}`}>
          <span className="plan-mark" aria-hidden>
            {t.status === "completed" ? <Icon name="check" size={10} /> : t.status === "in_progress" && live ? <Icon name="spinner" size={10} className="spin" /> : null}
          </span>
          <span>{t.status === "in_progress" && t.active ? t.active : t.content}</span>
          <span className="sr-only">{t.status === "completed" ? " (done)" : t.status === "in_progress" ? " (in hand)" : ""}</span>
        </li>
      ))}
    </ul>
  );
}
