import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import { answerCrew, type Crew, type CrewAnswer, type CrewPrompt, getCrewPrompt, getTranscript, sayToCrew, type Transcript, type TranscriptEntry, useVisible, type Workspace } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { type Call, Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { useDraft } from "../ui/useDraft";
import { pageSocket } from "../socket";
import { appliedTranscript } from "./transcript";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { ErrorCard } from "../ui/ErrorCard";
import { confirm } from "../ui/Confirm";
import { EditDiff } from "../ui/EditDiff";
import { Time } from "../ui/Time";
import { Todos } from "../ui/Todos";
import { CrewBadge } from "./parts";
import { useLoaded } from "../load";
import { useLatest } from "../load";
import { Button } from "../ui/Button";
import { MoreLink } from "../ui/MoreLink";
import { pageScroller } from "../ui/scroller";
import { Input } from "../ui/Input";
import { Composer } from "../ui/Composer";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { Turn, TurnAvatar, TurnHead, TurnSaid } from "../ui/Turn";
import { TypingDots } from "../ui/TypingDots";
import { useWorkspace } from "../workspace/store";

// The agent view: a crew as a conversation, the way in to it. What it was
// sent, what it said and the steps between, as it happens (the session
// file, followed); its plan pinned on top; what it's asking (a permission,
// a question) answered from here; and a box to tell it what's next, or to
// resume it with, once it's ended.

// useTranscript follows a crew's transcript over the tab's socket while the
// page is showing (transcript:<slug>/<crew>): read whole once it's
// followed, then told each change, which is applied to it as held
// (appliedTranscript), and read whole again only when one can't be. last:
// only its latest entries are held.
export function useTranscript(slug: string, id: string, last?: number): { t?: Transcript; error?: string; reload: () => void } {
  const [t, setT] = useState<Transcript>();
  // held is what's held, as of now: a change is applied to it as it's told,
  // not when the page next draws.
  const held = useRef<Transcript>(undefined);
  const put = useCallback((x: Transcript | undefined) => {
    held.current = x;
    setT(x);
  }, []);
  const [error, setError] = useState<string>();
  const visible = useVisible();
  const begin = useLatest();
  const reload = useCallback(() => {
    const fresh = begin();
    getTranscript(slug, id, last).then(
      (x) => {
        if (!fresh()) return;
        // A read that crossed a change told after it is older than what's held.
        if (held.current?.available && x.version < (held.current.version ?? 0)) return;
        put(x);
        setError(undefined);
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [slug, id, last, begin, put]);
  // Another crew's: a read of the last one's, under way, is stale, and
  // what's held is the last one's.
  useEffect(() => {
    begin();
    put(undefined);
  }, [slug, id, begin, put]);
  useEffect(() => {
    if (!visible) return;
    return pageSocket.follow(`transcript:${slug}/${id}`, {
      following: reload,
      changed: (said) => {
        const next = said.transcript ? appliedTranscript(held.current, said.transcript, last) : "read";
        if (next === "read") reload();
        else if (next) put(next);
      },
      lost: (closed) => closed && reload(),
    });
  }, [slug, id, last, visible, reload, put]);
  return { t, error, reload };
}

// A turn of the transcript: what crew was sent, what it said, or the steps
// between. Each is keyed by where it is in the whole session, so a fold
// stays open as the window of the latest entries slides.
type Group =
  | { kind: "brief"; at: number; text: string; time?: string }
  | { kind: "user"; at: number; text: string; time?: string }
  | { kind: "notice"; at: number; text: string }
  | { kind: "agent"; at: number; text: string; time?: string }
  | { kind: "calls"; at: number; calls: TranscriptEntry[] };

function group(t: Transcript): Group[] {
  const out: Group[] = [];
  const base = t.earlier ?? 0;
  t.entries.forEach((e, i) => {
    const at = base + i;
    const last = out[out.length - 1];
    if (e.by === "call") {
      if (last?.kind === "calls") last.calls.push(e);
      else out.push({ kind: "calls", at, calls: [e] });
      return;
    }
    const text = e.text ?? "";
    if (e.by === "agent") return out.push({ kind: "agent", at, text, time: e.at });
    // What crew's sent: ws's brief to it (its opening prompt), a harness's
    // notice, or you.
    if (/^You are crew on the mission|^You are the briefing agent|^You are an? .{0,40}(agent|crew)\b/.test(text) && at < 4) return out.push({ kind: "brief", at, text, time: e.at });
    if (/^\[Request interrupted|^\[.*\]$|^<command-|^Caveat:/.test(text.trim())) return out.push({ kind: "notice", at, text: text.trim().replace(/^\[|\]$/g, "") });
    out.push({ kind: "user", at, text, time: e.at });
  });
  return out;
}

const toCall = (c: TranscriptEntry): Call => ({
  tool: c.tool ?? "tool",
  input: c.input ?? "",
  result: c.result,
  error: c.error,
  refused: c.refused,
  pending: c.pending,
  edit: c.edit,
  todos: c.todos,
  questions: c.questions,
});

// pendingCall is the step crew are on, or waiting for permission for: the
// last call with nothing back yet.
export const pendingCall = (t?: Transcript) => [...(t?.entries ?? [])].reverse().find((e) => e.by === "call" && e.pending);

// capsuleFileLink makes an edited file's path a link into the capsule's
// file explorer, when it's in the crew's capsule.
function useFileLink(workspace: Workspace, slug: string, crew?: Crew) {
  return useMemo(() => {
    const m = workspace.missions.find((x) => x.slug === slug);
    const cap = m?.capsules.find((c) => `${c.repo}/${c.name}` === crew?.capsule);
    if (!cap) return undefined;
    return (path: string) => {
      const prefix = cap.path.replace(/\/$/, "") + "/";
      const rel = path.startsWith(prefix) ? path.slice(prefix.length) : !path.startsWith("/") ? path : undefined;
      return rel ? `/missions/${slug}/files/${cap.repo}/${cap.name}?${new URLSearchParams({ path: rel })}` : undefined;
    };
  }, [workspace, slug, crew?.capsule]);
}

export function AgentView({ slug, id, crew, live, onTerminal }: { slug: string; id: string; crew?: Crew; live: boolean; onTerminal?: () => void }) {
  const workspace = useWorkspace();
  const { t, error, reload } = useTranscript(slug, id);
  const [unseen, setUnseen] = useState(false);
  const follow = useRef(true);
  const first = useRef(true);
  const fileLink = useFileLink(workspace, slug, crew);
  const root = workspace.root;

  useEffect(() => {
    const m = pageScroller();
    if (!m) return;
    const onScroll = () => {
      follow.current = m.scrollHeight - m.scrollTop - m.clientHeight < 160;
      if (follow.current) setUnseen(false);
    };
    m.addEventListener("scroll", onScroll, { passive: true });
    return () => m.removeEventListener("scroll", onScroll);
  }, []);
  const size = (t?.earlier ?? 0) + (t?.entries.length ?? 0);
  const asking = live && (crew?.activity === "permission" || crew?.activity === "question");
  // Open on the latest; then follow it, unless you've scrolled up to read.
  useLayoutEffect(() => {
    if (!t) return;
    const m = pageScroller();
    if (!m) return;
    if (first.current || follow.current) {
      m.scrollTo({ top: m.scrollHeight, behavior: first.current ? "auto" : "smooth" });
      first.current = false;
    } else setUnseen(true);
  }, [size, t, asking]);

  if (error && !t) return <ErrorCard title="Couldn't read the transcript" error={error} retry={reload} />;
  if (!t) return <Loading what="Reading the transcript…" />;
  if (!t.available && !live)
    return (
      <Empty icon="message" title="No transcript">
        {t.why}
      </Empty>
    );
  const groups = group(t);
  const plan = [...t.entries].reverse().find((e) => e.todos?.length)?.todos;
  const pending = pendingCall(t);
  const working = live && crew?.activity === "working";
  return (
    <div className="agent-view">
      {plan && <PinnedPlan todos={plan} live={live} />}
      {!t.available && (
        <Empty icon="terminal" title="No transcript to show" action={onTerminal && <Button onClick={onTerminal} keys="t">Watch its terminal</Button>}>
          {t.why} You can still send it a message below.
        </Empty>
      )}
      <div className="talk transcript">
        {!!t.earlier && <div className="transcript-earlier">{t.earlier} earlier entries aren't shown.</div>}
        {groups.map((g, i) => {
          const latest = i === groups.length - 1;
          switch (g.kind) {
            case "brief":
              return (
                <details key={g.at} className="transcript-brief">
                  <summary>
                    <Icon name="chevron-right" size={12} className="fold-chevron" />
                    <TurnAvatar by="agent" className="brief-avatar" />
                    ws briefed {id}: the mission, and how to report back
                    {g.time && <Time iso={g.time} className="dim" />}
                  </summary>
                  <div className="said-text">{g.text}</div>
                </details>
              );
            case "notice":
              return (
                <div key={g.at} className="transcript-notice">
                  <Icon name="info" size={12} /> {g.text.replace(/^Request interrupted by user.*$/, "Interrupted")}
                </div>
              );
            case "calls":
              return (
                <div key={g.at} className="transcript-calls">
                  <Calls calls={g.calls.map(toCall)} open={live && latest} live={working} root={root} fileLink={fileLink} />
                </div>
              );
            default:
              return (
                <Turn
                  key={g.at}
                  by={g.kind === "user" ? "you" : "agent"}
                  head={<TurnHead avatar={g.kind === "user" ? <TurnAvatar by="you" /> : <CrewBadge id={id} />} who={g.kind === "user" ? "Sent" : undefined} at={g.time} />}
                >
                  <TurnSaid>
                    <Markdown text={g.text} />
                  </TurnSaid>
                </Turn>
              );
          }
        })}
        {!t.entries.length && t.available && (
          <Empty icon="message" title="Nothing yet">
            What {id} is sent, says and does shows here as it happens.
          </Empty>
        )}
        {working && !asking && (
          <div className="typing agent-working">
            <TypingDots />
            {pending ? (
              <>
                {pending.tool === "Bash" ? "Running" : "Working on"} <code>{pending.edit?.path.split("/").pop() ?? pending.input?.slice(0, 80)}</code>
              </>
            ) : (
              "Working"
            )}
          </div>
        )}
      </div>
      <div className="agent-dock">
        {unseen && (
          <button
            className="new-pill"
            onClick={() => {
              const m = pageScroller();
              m?.scrollTo({ top: m.scrollHeight, behavior: "smooth" });
              setUnseen(false);
            }}
          >
            <Icon name="arrow-down" size={12} /> Newer
          </button>
        )}
        {asking && crew && <PromptCard slug={slug} crew={crew} pending={pending} fileLink={fileLink} onTerminal={onTerminal} />}
        <TellCrew slug={slug} id={id} crew={crew} live={live} working={working} />
      </div>
    </div>
  );
}

// PinnedPlan is crew's latest plan, on top: how far through it is, and
// (opened) each item.
function PinnedPlan({ todos, live }: { todos: NonNullable<TranscriptEntry["todos"]>; live: boolean }) {
  const done = todos.filter((t) => t.status === "completed").length;
  const now = todos.find((t) => t.status === "in_progress");
  return (
    <details className="pinned-plan" open>
      <summary>
        <Icon name="chevron-right" size={12} className="fold-chevron" />
        <span className="pinned-plan-title">Plan</span>
        <span className="plan-progress" aria-hidden>
          <span style={{ width: `${(100 * done) / todos.length}%` }} />
        </span>
        <span className="dim num">
          {done} of {todos.length}
        </span>
        {now && <span className="pinned-plan-now">{now.active || now.content}</span>}
      </summary>
      <Todos todos={todos} live={live} />
    </details>
  );
}

// PromptCard is what crew are stopped on: a permission (the step it wants
// to take, and allow or deny) or a question (its options, or your own
// words). Answering types the keys a person would, once the server's read
// the screen and seen the same prompt; if it hasn't, the terminal's the
// place.
export function PromptCard({
  slug,
  crew,
  pending,
  fileLink,
  onTerminal,
  compact,
}: {
  slug: string;
  crew: Crew;
  pending?: TranscriptEntry;
  fileLink?: (path: string) => string | undefined;
  onTerminal?: () => void;
  compact?: boolean;
}) {
  const [busy, setBusy] = useState(false);
  const [own, setOwn] = useState("");
  // What it's asking about, read off its screen, when the transcript
  // hasn't the step yet (a harness can write it late).
  const { value: screen } = useLoaded<CrewPrompt | undefined>(
    crew.activity === "permission" && !pending ? () => getCrewPrompt(slug, crew.id).catch(() => undefined) : null,
    [slug, crew.id, crew.activity, crew.notice, pending],
  );
  const about = screen?.context.filter((l) => !/^tip:/i.test(l));
  const answer = async (a: CrewAnswer, ask?: { title: string; body: string; confirm: string }) => {
    if (ask && !(await confirm({ ...ask }))) return;
    setBusy(true);
    answerCrew(slug, crew.id, a)
      .then(
        (r) => toast(r.message, { tone: "done" }),
        (e: Error) =>
          toast(humanError(e.message), {
            tone: "danger",
            ms: 8000,
            action: onTerminal ? { label: "Open the terminal", run: onTerminal } : undefined,
          }),
      )
      .finally(() => setBusy(false));
  };
  // Crew over ACP have no terminal: the card is the only way to answer.
  const terminal = crew.structured ? null : onTerminal ? (
    <MoreLink onClick={onTerminal} keys="t">
      Answer in the terminal
    </MoreLink>
  ) : (
    <MoreLink to={`/missions/${slug}/crew/${crew.id}?view=terminal`}>
      Answer in the terminal
    </MoreLink>
  );
  if (crew.activity === "permission") {
    const link = pending?.edit && fileLink?.(pending.edit.path);
    return (
      <div className={`prompt-card tone-danger${compact ? " compact" : ""}`} role="group" aria-label={`${crew.id} needs your permission`}>
        <div className="prompt-head">
          <Icon name="lock" size={14} />
          <span>
            <strong>{crew.id}</strong> wants to{" "}
            {pending ? (pending.edit ? (pending.edit.old ? "edit" : "write") : pending.tool === "Bash" ? "run" : `use ${pending.tool}`) : about?.length ? (screen?.title.replace(/^.*?(Do you want to [^?]*)\?.*$/i, "$1").replace(/^Do you want to /i, "") || "go on") : "go on"}
            {pending?.edit && <code className="prompt-path">{pending.edit.path.split("/").slice(-2).join("/")}</code>}
          </span>
        </div>
        {pending && !pending.edit && pending.input && <pre className="prompt-input">{pending.input}</pre>}
        {pending?.edit && !compact && <EditDiff edit={pending.edit} max={10} />}
        {link && !compact && (
          <Link to={link} className="call-file-link">
            <Icon name="file" size={12} /> The file as it is
          </Link>
        )}
        {!pending && !!about?.length && <pre className="prompt-input">{about.join("\n")}</pre>}
        {!pending && !about?.length && crew.notice && <div className="dim">{crew.notice}</div>}
        <div className="prompt-actions">
          <Button variant="primary" disabled={busy} onClick={() => answer({ permission: "allow" })}>
            <Icon name="check" size={13} /> Allow once
          </Button>
          <Button
            disabled={busy}
            onClick={() =>
              answer(
                { permission: "always" },
                {
                  title: "Allow it from now on?",
                  body: `${crew.id} won't ask again for this kind of thing, for the rest of its session. Its harness decides what "this kind" covers.`,
                  confirm: "Allow from now on",
                },
              )
            }
          >
            Always allow…
          </Button>
          <Button variant="ghost" disabled={busy} onClick={() => answer({ permission: "deny" })}>
            Deny
          </Button>
          <span className="spacer" />
          {terminal}
        </div>
      </div>
    );
  }
  // A question, with its own options, or your words.
  const q = pending?.questions?.[0];
  const options = crew.options?.length ? crew.options : (q?.options ?? []);
  return (
    <div className={`prompt-card tone-attention${compact ? " compact" : ""}`} role="group" aria-label={`${crew.id} asks you`}>
      <div className="prompt-head">
        <Icon name="question" size={14} />
        <span>
          <strong>{crew.id}</strong> asks: {crew.notice || q?.question || "a question"}
        </span>
      </div>
      {options.length > 0 && (
        <div className="prompt-options">
          {options.map((o) => (
            <Button key={o} disabled={busy} onClick={() => answer({ option: o })}>
              {o}
            </Button>
          ))}
        </div>
      )}
      <form
        className="own-answer"
        onSubmit={(e) => {
          e.preventDefault();
          if (own.trim()) answer({ text: own });
        }}
      >
        <Input value={own} onChange={(e) => setOwn(e.target.value)} placeholder="Or in your own words" aria-label="Your answer" disabled={busy} />
        <Button variant="primary" type="submit" disabled={busy || !own.trim()}>
          Answer
        </Button>
      </form>
      <div className="prompt-actions">
        <span className="spacer" />
        {terminal}
      </div>
    </div>
  );
}

// TellCrew tells crew what's next: typed in and sent if it's running, or
// resuming it with it if it's ended. While it works, Interrupt stops its
// turn (Escape), not the crew.
function TellCrew({ slug, id, crew, live, working }: { slug: string; id: string; crew?: Crew; live: boolean; working: boolean }) {
  const [text, setText] = useDraft(`crew:${slug}/${id}`);
  const [busy, setBusy] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  const canResume = !live && !!crew && crew.ended;
  // The key that sends, as the box has it (ui/Composer.tsx): the button's too.
  const sendKey = useSendKey();
  const send = () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    sayToCrew(slug, id, text)
      .then(
        (r) => {
          setText("");
          toast(r.message, { tone: "done" });
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };
  const interrupt = () =>
    answerCrew(slug, id, { interrupt: true }).then(
      (r) => toast(r.message),
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  if (!live && !canResume) return null;
  return (
    <Composer
      className="agent-composer"
      field={box}
      value={text}
      onChange={setText}
      onSend={send}
      tallest={280}
      aria-label={`Message ${id}`}
      placeholder={live ? (working ? `Tell ${id} something: it reads it when this step's done` : `Tell ${id} what's next…`) : `Resume ${id} with a message…`}
      hint={
        live && crew?.structured ? (
          <>
            <Icon name="message" size={12} /> Sent to it as a message{working ? ", for when this turn ends" : ""}.
          </>
        ) : live ? (
          <>
            <Icon name="terminal" size={12} /> Typed into its terminal and sent, as if you'd typed it there.
          </>
        ) : (
          <>
            <Icon name="rotate" size={12} /> It's ended: this starts it again, in its capsule, with your message.
          </>
        )
      }
      also={
        working && (
          <Button variant="ghost" onClick={interrupt} title="Stop what it's doing now (its turn), not the crew">
            <Icon name="stop" size={12} /> Interrupt
          </Button>
        )
      }
    >
      <Button variant="primary" disabled={!text.trim() || busy} onClick={send} keys={sendKeys(sendKey)}>
        {busy ? "Sending…" : live ? "Send" : "Resume"}
      </Button>
    </Composer>
  );
}
