import { lazy, Suspense, useCallback, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { act } from "../../api";
import { Icon } from "../../ui/Icon";
import { useKeys } from "../../keys";
import { useTitle } from "../../ui/useTitle";
import { toast } from "../../ui/toast";
import { Loading } from "../../ui/Loading";
import { Empty } from "../../ui/Empty";
import { humanError } from "../../ui/strings";
import { confirm } from "../../ui/Confirm";
import type { StatusInfo } from "../../ui/Status";
import { crewStatus } from "../../ui/statusOf";
import type { TermState } from "./CrewTerminal";
import { CrewConversation } from "./CrewConversation";
import { CrewModelSwitch } from "./CrewModelSwitch";
import { MissionCrewHead } from "./MissionCrewHead";
import { MissionCrewTerminal } from "./MissionCrewTerminal";
import { CrewBadge } from "../CrewBadge";
import { Count } from "../../ui/Count";
import { Button } from "../../ui/Button";
import { Segment, Segmented } from "../../ui/Segmented";
import { useWorkspace } from "../../workspace/store";
import { PagePin } from "../../shell/PagePin";
import { MissionFrame, MissionFrameBody, MissionFrameMain } from "../Frame/MissionFrame";
import { MissionBar, type MissionTab } from "../Frame/MissionBar";
import { MissionTimeline } from "../MissionTimeline";
import { useTimeline } from "../timeline";
import { useMissionDetail } from "../useMissionDetail";

const CrewTerminal = lazy(() => import("./CrewTerminal"));

export const crewKeys: [string, string][] = [
  ["r", "Message crew"],
  ["mod+⏎", "Send"],
  ["t", "Terminal / conversation"],
  ["(any key)", "In terminal while focused, Esc included"],
  ["mod+C / mod+V", "Copy / paste"],
  ["(click outside)", "Leave terminal"],
];

// CrewPage is crew at work: first as a conversation (the agent view: what
// it's doing, what it asks, a box to tell it what's next), and its
// terminal, for anything the conversation can't do. It's a page under its
// mission: the mission's timeline and bar on top (Work lit, where its crew
// are), then the crew's own bar, which stays, and its conversation or
// terminal scrolling under them.
export function CrewPage() {
  const { slug = "", id = "" } = useParams();
  const m = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const listed = m?.crew.find((c) => c.id === id);
  // The mission, for its bar and timeline. An archived mission's crew
  // aren't in the workspace's list: it says them too.
  const { value: d } = useMissionDetail(slug);
  const c = listed ?? d?.crew.find((c) => c.id === id);
  const strip = useTimeline(d);
  const navigate = useNavigate();
  const toTab = (t: MissionTab) => navigate(t === "summary" ? `/missions/${slug}` : `/missions/${slug}?tab=${t}`);
  const missionTitle = d?.title ?? m?.title;
  useTitle(`${id}${missionTitle ? ` · ${missionTitle}` : ""}`);
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
      body: "Current work is lost. The session can be resumed later, if recorded.",
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
  const asking = state === "live" && (c?.activity === "permission" || c?.activity === "question");
  return (
    <MissionFrame>
      {strip && <MissionTimeline {...strip} />}
      <MissionBar
        d={d}
        title={m?.title}
        headline={m?.headline}
        tab="work"
        onTab={toTab}
        under
        back={{ to: `/missions/${slug}?tab=work`, title: missionTitle ?? "Mission", label: "Back to the mission" }}
      >
        <PagePin r={{ kind: "mission", slug }} />
      </MissionBar>
      <MissionCrewHead
        name={c?.role === "briefing" ? `Briefing agent ${id}` : <CrewBadge id={id} live={state === "live"} />}
        status={st}
        runs={
          c &&
          (state === "live" && c.switchModel ? (
            <>
              {c.role === "briefing" ? "briefing on" : c.harness} <CrewModelSwitch slug={slug} crew={c} />
            </>
          ) : c.role === "briefing" ? (
            "briefing"
          ) : c.model ? (
            `${c.harness} on ${c.model}`
          ) : (
            c.harness
          ))
        }
        acp={structured}
        capsule={c?.capsule}
        termTitle={title}
        view={
          !structured && (
            <Segmented size="small" value={shown} onChange={setView} aria-label="Crew">
              {(["conversation", "terminal"] as const).map((v) => (
                <Segment key={v} value={v} keys="t" title={v === "terminal" ? "Terminal" : "Conversation"}>
                  <Icon name={v === "terminal" ? "terminal" : "message"} size={13} />
                  {v === "terminal" ? "Terminal" : "Conversation"}
                  {v === "conversation" && asking && <Count tone="attention">1</Count>}
                </Segment>
              ))}
            </Segmented>
          )
        }
        actions={
          state === "live" && (
            <Button size="small" onClick={stop}>
              <Icon name="stop" size={12} /> Stop
            </Button>
          )
        }
      />
      <MissionFrameBody>
        <MissionFrameMain>
          {shown === "conversation" && (
            <CrewConversation slug={slug} id={id} crew={c} live={state === "live"} onTerminal={state === "live" && !structured ? () => setView("terminal") : undefined} field={tell} />
          )}
          {!structured && <div hidden={shown === "conversation"}>
            {state === "gone" ? (
              <Empty
                icon="terminal"
                title={`${id} isn't running`}
                action={
                  <Button onClick={() => setView("conversation")} keys="t">
                    <Icon name="message" /> Open conversation
                  </Button>
                }
              >
                No terminal: {id} isn't running.
              </Empty>
            ) : (
              <Suspense fallback={<Loading what="Opening the terminal…" />}>
                <MissionCrewTerminal id={id} state={state === "live" || state === "ended" ? state : undefined}>
                  <CrewTerminal slug={slug} id={id} onState={onState} onTitle={onTitle} />
                </MissionCrewTerminal>
              </Suspense>
            )}
          </div>}
        </MissionFrameMain>
      </MissionFrameBody>
    </MissionFrame>
  );
}
