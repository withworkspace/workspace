import { lazy, Suspense, useCallback, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { act, getMission } from "../api";
import { useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Crumbs } from "../ui/Crumbs";
import { useKeys } from "../keys";
import { Empty, humanError, Keys, Loading, toast, useTitle } from "../ui/kit";
import { confirm } from "../ui/Confirm";
import { crewStatus, Status, type StatusInfo } from "../ui/status";
import type { TermState } from "./CrewTerminal";
import { AgentView } from "./AgentView";
import { CrewBadge } from "./parts";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";
import { Tab, Tabs } from "../ui/Tabs";
import { useWorkspace } from "../workspace/store";

const CrewTerminal = lazy(() => import("./CrewTerminal"));

export const crewKeys: [string, string][] = [
  ["r", "Write to the crew"],
  ["mod+⏎", "Send it"],
  ["t", "The terminal, or back to the conversation"],
  ["(any key)", "In the terminal, while it has focus: goes to the crew, Esc included"],
  ["mod+C / mod+V", "Copy what's selected / paste into the crew"],
  ["(click outside)", "Stop typing into it"],
];

// CrewPage is crew at work: first as a conversation (the agent view: what
// it's doing, what it asks, a box to tell it what's next), and its
// terminal, for anything the conversation can't do.
export function CrewPage() {
  const { slug = "", id = "" } = useParams();
  const m = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const listed = m?.crew.find((c) => c.id === id);
  // An archived mission's crew aren't in the workspace's list: its own
  // page says them.
  const detail = useLoaded(listed || !m ? null : () => getMission(slug), [slug, !!listed]);
  const c = listed ?? detail.value?.crew.find((c) => c.id === id);
  useTitle(`${id}${m ? ` · ${m.title}` : ""}`);
  const [termState, setState] = useState<TermState>("connecting");
  // Crew over ACP have no terminal: they're live while their record says
  // they run, and their conversation is all there is.
  const structured = !!c?.structured;
  const state: TermState = structured ? (c?.ended ? "ended" : "live") : termState;
  const [title, setTitle] = useState("");
  const onState = useCallback((s: TermState) => setState(s), []);
  const onTitle = useCallback((t: string) => setTitle(t), []);
  const [params, setParams] = useSearchParams();
  const shown = params.get("view") === "terminal" && !structured ? "terminal" : "conversation";
  const setView = (v: "terminal" | "conversation") => setParams(v === "terminal" ? { view: "terminal" } : {}, { replace: true });
  useKeys({
    t: () => !structured && setView(shown === "terminal" ? "conversation" : "terminal"),
    r: () => {
      if (shown !== "conversation") setView("conversation");
      requestAnimationFrame(() => document.querySelector<HTMLTextAreaElement>(".agent-composer textarea")?.focus());
    },
  });
  const stop = async () => {
    const yes = await confirm({
      title: `Stop ${id}?`,
      body: "Its session can be resumed later if it recorded one: what it's in the middle of is lost.",
      confirm: `Stop ${id}`,
      danger: true,
    });
    if (!yes) return;
    act(slug, "stop", { crew: id }).then(
      (r) => toast(r.message, { tone: "done" }),
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );
  };
  const st: StatusInfo =
    state === "live" && c
      ? crewStatus(c)
      : state === "connecting"
        ? { icon: "spinner", tone: "quiet", words: "Connecting", live: true }
        : { icon: "circle", tone: "quiet", words: "Not running" };
  return (
    <div className="page crew-page">
      <Crumbs steps={[{ to: `/missions/${slug}`, label: m?.title ?? slug }]} />
      <header className="crew-head">
        <div className="crew-head-title">
          <h1>
            {c?.role === "briefing" ? `Briefing agent ${id}` : <CrewBadge id={id} live={state === "live"} large />}
            <Status s={st} className="crew-head-status" />
          </h1>
          <div className="crew-head-sub">
            {c && <span>{c.role === "briefing" ? "briefing" : c.model ? `${c.harness} on ${c.model}` : c.harness}</span>}
            {structured && <span className="dim">over ACP, no terminal</span>}
            {c?.capsule && (
              <span className="mono">
                <Icon name="box" size={12} /> {c.capsule}
              </span>
            )}
            {title && <span className="mono term-title" title="What its terminal calls itself">{title}</span>}
          </div>
        </div>
        {state === "live" && (
          <Button onClick={stop}>
            <Icon name="stop" size={12} /> Stop
          </Button>
        )}
      </header>
      {!structured && <Tabs value={shown} onChange={setView} aria-label="Crew">
        {(["conversation", "terminal"] as const).map((v) => (
          <Tab key={v} value={v}>
            <Icon name={v === "terminal" ? "terminal" : "message"} size={13} />
            {v === "terminal" ? "Terminal" : "Conversation"}
            {v === "conversation" && state === "live" && (c?.activity === "permission" || c?.activity === "question") && <Count tone="attention">1</Count>}
          </Tab>
        ))}
      </Tabs>}
      {shown === "conversation" && (
        <AgentView slug={slug} id={id} crew={c} live={state === "live"} onTerminal={state === "live" && !structured ? () => setView("terminal") : undefined} />
      )}
      {!structured && <div className="term-panel" hidden={shown === "conversation"}>
        {state === "gone" ? (
          <Empty
            icon="terminal"
            title={`${id} isn't running`}
            action={
              <Button onClick={() => setView("conversation")}>
                <Icon name="message" /> Read its conversation
              </Button>
            }
          >
            There's no terminal to show: crew run in the workspace's daemon, and {id} isn't running there now.
          </Empty>
        ) : (
          <Suspense fallback={<Loading what="Opening the terminal…" />}>
            <div className="term-frame">
              <CrewTerminal slug={slug} id={id} onState={onState} onTitle={onTitle} />
              {state === "live" && (
                <div className="term-bar">
                  <span className="term-focus">
                    <Icon name="keyboard" size={12} /> Typing goes to {id} while the terminal has focus, <Keys k="esc" /> included. Click outside to stop.
                  </span>
                  <span className="dim">
                    <Keys k="mod+C" /> copies, <Keys k="mod+V" /> pastes
                  </span>
                </div>
              )}
              {state === "ended" && (
                <div className="term-bar ended">
                  <Icon name="info" size={12} /> {id} has ended: this is its last screen.
                </div>
              )}
            </div>
          </Suspense>
        )}
      </div>}
    </div>
  );
}
