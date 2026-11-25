import { lazy, Suspense, useCallback, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { act, getMission } from "../../api";
import { useLoaded } from "../../load";
import { Icon } from "../../ui/Icon";
import { Crumbs } from "../../ui/Crumbs";
import { useKeys } from "../../keys";
import { useTitle } from "../../ui/useTitle";
import { toast } from "../../ui/toast";
import { Loading } from "../../ui/Loading";
import { Empty } from "../../ui/Empty";
import { humanError } from "../../ui/text";
import { confirm } from "../../ui/Confirm";
import type { StatusInfo } from "../../ui/Status";
import { crewStatus } from "../../ui/statusOf";
import type { TermState } from "./CrewTerminal";
import { CrewConversation } from "./CrewConversation";
import { MissionCrewHead } from "./MissionCrewHead";
import { MissionCrewPage } from "./MissionCrewPage";
import { MissionCrewTerminal } from "./MissionCrewTerminal";
import { CrewBadge } from "../CrewBadge";
import { Count } from "../../ui/Count";
import { Button } from "../../ui/Button";
import { Tab, Tabs } from "../../ui/Tabs";
import { useWorkspace } from "../../workspace/store";

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
  // The box to write to crew, which r gives the keys.
  const tell = useRef<HTMLTextAreaElement>(null);
  useKeys({
    t: () => !structured && setView(shown === "terminal" ? "conversation" : "terminal"),
    r: () => {
      if (shown !== "conversation") setView("conversation");
      requestAnimationFrame(() => tell.current?.focus());
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
    <MissionCrewPage>
      <Crumbs steps={[{ to: `/missions/${slug}`, label: m?.title ?? slug }]} />
      <MissionCrewHead
        name={c?.role === "briefing" ? `Briefing agent ${id}` : <CrewBadge id={id} live={state === "live"} large />}
        status={st}
        runs={c && (c.role === "briefing" ? "briefing" : c.model ? `${c.harness} on ${c.model}` : c.harness)}
        acp={structured}
        capsule={c?.capsule}
        termTitle={title}
        actions={
          state === "live" && (
            <Button onClick={stop}>
              <Icon name="stop" size={12} /> Stop
            </Button>
          )
        }
      />
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
        <CrewConversation slug={slug} id={id} crew={c} live={state === "live"} onTerminal={state === "live" && !structured ? () => setView("terminal") : undefined} field={tell} />
      )}
      {!structured && <div hidden={shown === "conversation"}>
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
            <MissionCrewTerminal id={id} state={state === "live" || state === "ended" ? state : undefined}>
              <CrewTerminal slug={slug} id={id} onState={onState} onTitle={onTitle} />
            </MissionCrewTerminal>
          </Suspense>
        )}
      </div>}
    </MissionCrewPage>
  );
}
