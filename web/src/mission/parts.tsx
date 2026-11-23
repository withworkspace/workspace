import { createContext, Fragment, type ReactNode, useContext } from "react";
import { Link } from "react-router";
import type { Artifact, CapsuleDetail, CrewDetail, MissionDetail, MissionEvent, Stage } from "../api";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { HoverCard } from "../ui/HoverCard";
import { Copyable } from "../ui/Copyable";
import { Status } from "../ui/Status";
import { crewStatus, exitWords } from "../ui/statusOf";
import { Time } from "../ui/Time";
import { dayKey, dayLabel, duration, useNow } from "../ui/when";
import { Chip, chipClass } from "../ui/Chip";
import { crewHue } from "./crewHue";
import { MissionCrewBadge } from "./MissionCrewBadge";
import { MissionCrewCard } from "./MissionCrewCard";
import { MissionPlan } from "./MissionPlan";
import { MissionPlanCheckList, type StepCheck } from "./MissionPlanCheckList";
import { MissionPlanNote } from "./MissionPlanNote";
import { MissionPlanStep } from "./MissionPlanStep";
import { type Tone } from "../ui/tone";
import { capital } from "../ui/text";

// The pieces a mission's page draws with: who did what (events), stages,
// decisions, documents and capsules.

export { crewHue };

// MissionData is the mission a page is about, for what draws a ref (a crew's
// badge) to say more about it on hover.
export const MissionData = createContext<MissionDetail | undefined>(undefined);

// CrewBadge is a mission's crew by name, which says more of it on hover
// where the page knows the mission: who it is, where it stands, what it
// last said.
export function CrewBadge({ id, live, large }: { id: string; live?: boolean; large?: boolean }) {
  const d = useContext(MissionData);
  const c = d?.crew.find((x) => x.id === id);
  const badge = <MissionCrewBadge id={id} live={live} large={large} />;
  if (!c) return badge;
  return (
    <HoverCard trigger={badge}>
      <MissionCrewCard
        badge={<MissionCrewBadge id={id} live={live} />}
        status={<Status s={crewStatus(c)} />}
        what={c.role === "briefing" ? "Briefing agent" : c.model ? `${c.harness} on ${c.model}` : c.harness}
        capsule={c.capsule}
        started={<Time iso={c.started} />}
        ended={c.endedAt && <Time iso={c.endedAt} />}
        said={(c.note || c.lastReply) && <Inline text={c.note || c.lastReply || ""} lines={3} />}
        to={d && `/missions/${d.slug}/crew/${c.id}`}
        more={c.ended ? "Read its transcript" : "Open its terminal"}
      />
    </HoverCard>
  );
}

// Who did something in the log.
export type ActorKind = "you" | "crew" | "orchestrator" | "agent" | "ws";

export interface EventView {
  actor: ActorKind;
  who: string;
  crew?: string;
  text: string; // the rest of the sentence
  title?: string; // a tooltip: the words replaced
}

// viewEvent splits an event's sentence into who and what they did, and
// says crew ending in words ("its terminal closed", not "exit 129").
export function viewEvent(e: MissionEvent): EventView {
  let t = e.text.trim();
  let title: string | undefined;
  // (One logged before ws said how it ended: stopped, or by itself.)
  t = t.replace(/ended \((stopped: )?(exit \d+)\)/, (_, stopped: string | undefined, code: string) => {
    title = code;
    return stopped ? "was stopped" : `ended: ${exitWords(code)}`;
  });
  t = t.replace(/^note: /, "noted ");
  // A message to crew (to resume with, or typed in): the crew it names is
  // whom it's for; by says who said it. One logged before ws said was the
  // orchestrator's (you were never logged).
  if (e.kind === "resume_with" || e.kind === "told") {
    const by = e.to ? e.by : undefined;
    if (by === "you" || (!by && e.kind === "told")) return { actor: "you", who: "You", text: t, title };
    if (by === "ws") return { actor: "ws", who: "ws", text: t, title };
    return { actor: "orchestrator", who: "Orchestrator", text: t, title };
  }
  if (e.by === "orchestrator" || /^orchestrator( failed)?: /.test(t)) {
    const failed = /^orchestrator failed: /.test(t);
    return { actor: "orchestrator", who: "Orchestrator", text: (failed ? "failed: " : "") + t.replace(/^orchestrator( failed)?: /, ""), title };
  }
  const crew = e.by && /^[a-z]+\d+$/.test(e.by) ? e.by : /^(c\d+) /.exec(t)?.[1];
  if (crew) return { actor: "crew", who: crew, crew, text: t.startsWith(crew + " ") ? t.slice(crew.length + 1) : t, title };
  if (/^you /.test(t)) return { actor: "you", who: "You", text: t.slice(4), title };
  if (/^agent /.test(t)) return { actor: "agent", who: "Agent", text: t.slice(6), title };
  return { actor: "ws", who: "", text: t, title };
}

const actorIcon: Record<ActorKind, IconName> = { you: "user", crew: "terminal", orchestrator: "sparkle", agent: "message", ws: "circle" };

function Actor({ v }: { v: EventView }) {
  if (v.actor === "crew" && v.crew) return <CrewBadge id={v.crew} />;
  return (
    <span className={`actor actor-${v.actor}`} title={v.who || "ws"}>
      <Icon name={actorIcon[v.actor]} size={12} />
    </span>
  );
}

// Events is a stretch of the log: by day, each line who and what, its time
// on the right. lines clamps each (a summary); without, they're whole.
export function Events({ events, lines, days = true }: { events: MissionEvent[]; lines?: number; days?: boolean }) {
  const now = useNow();
  const spansDays = days && new Set(events.map((e) => dayKey(e.time))).size > 1;
  let lastDay = "";
  return (
    <ol className="events">
      {events.map((e, i) => {
        const v = viewEvent(e);
        const day = dayKey(e.time);
        const head = spansDays && day !== lastDay;
        lastDay = day;
        return (
          <Fragment key={i}>
            {head && (
              <li className="event-day" aria-hidden>
                {dayLabel(e.time, now)}
              </li>
            )}
            <li className={`event by-${v.actor} kind-${e.kind}`}>
              <Actor v={v} />
              <span className="event-text" title={v.title}>
                {v.who && v.actor !== "crew" && <span className="event-who">{v.who} </span>}
                <Inline text={v.text} lines={lines} />
              </span>
              <Time iso={e.time} as={spansDays ? "clock" : "stamp"} className="event-time" />
            </li>
          </Fragment>
        );
      })}
    </ol>
  );
}

// Stepper is a mission's stages: done ✓, the current one, and those to
// come, with who's on each and whether to check in after.
// StepWho is who's on a step of the plan that isn't crew's: yours (a
// checkpoint, which only you pass: a review, a decision, a landing), ws's
// (it opens the pull request), CI's, someone outside's, the planner's.
function StepWho({ s }: { s: Stage }) {
  const yours: Record<string, [string, string]> = {
    Decide: ["yours, to decide", "Yours to decide: your answer is recorded as a decision, and passes the step"],
    Land: ["yours, to land", "Yours to land: the step passes once it's landed"],
  };
  const [icon, label, title]: [IconName, string, string] =
    s.who === "you"
      ? ["user", ...(yours[s.type ?? ""] ?? ["yours", "A checkpoint: you look at what the steps before it made, and only you pass it"])]
      : s.who === "ws"
        ? ["pr", "ws opens it", "ws opens the pull request itself, and passes the step when it merges"]
        : s.who === "the planner"
          ? ["sparkle", "planner", "The planner's: it writes the steps after it"]
          : ["clock", s.who === "CI" ? "CI" : "outside", `Waited for: ${s.whoSays ?? s.who}`];
  return (
    <MissionPlanNote icon={icon} yours={s.who === "you"} title={title}>
      {label}
    </MissionPlanNote>
  );
}

// StepRuns is where a step that runs in several repos runs: the repos
// once they're named, each done or not, or what its words say before.
export function StepRuns({ s }: { s: Stage }) {
  const runs = s.runs ?? [];
  if (!runs.length) {
    return (
      <MissionPlanNote icon="layers" title={`In each ${s.inEach}: the orchestrator names the repos when it starts`}>
        in each {s.inEach}
      </MissionPlanNote>
    );
  }
  const done = runs.filter((r) => r.done).length;
  return (
    <MissionPlanNote icon="layers" title={runs.map((r) => `${r.repo}${r.done ? ": done" : ""}`).join("\n")}>
      {runs.length === done ? `in ${runs.length} repos` : `${done} of ${runs.length} repos done`}
    </MissionPlanNote>
  );
}

// StepLimit is a Work step's clock: its limit, when it runs out, or that
// it has.
export function StepLimit({ s }: { s: Stage }) {
  if (s.outOfTime) {
    return (
      <MissionPlanNote icon="clock" late title={`Its limit (${s.limit}) ran out while its crew were at it`}>
        out of time
      </MissionPlanNote>
    );
  }
  const at = s.deadline ? new Date(s.deadline) : null;
  return (
    <MissionPlanNote icon="clock" title={`Its limit: ${s.limit}. ws holds the clock from when crew start on it.`}>
      {at ? `until ${at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : (s.limit ?? "").split(/[,;]/)[0]}
    </MissionPlanNote>
  );
}

// StepChecks is how many of a Work step's checks are met, or how many it
// has before they've been run.
export function StepChecks({ s }: { s: Stage }) {
  const checks = s.checks ?? [];
  const met = checks.filter((c) => c.result === "met").length;
  // A Verify step is held up by what's unmet: what its verifier left for
  // you (can't tell, yours) is yours when you look at its bundle.
  const short = s.type === "Verify" ? checks.some((c) => c.result === "unmet") : met < checks.length;
  const title = checks.map((c) => `${c.result ? `${c.result}: ` : ""}${c.text}`).join("\n");
  if (!s.round) {
    return (
      <MissionPlanNote icon="list" title={`Done when these are true:\n${title}`}>
        {checks.length} {checks.length === 1 ? "check" : "checks"}
      </MissionPlanNote>
    );
  }
  return (
    <MissionPlanNote icon={short ? "alert" : "check"} late={short} title={`Round ${s.round}:\n${title}`}>
      {met} of {checks.length} met
    </MissionPlanNote>
  );
}

// checksOf is what a Work step's checks came to in their latest round,
// those not met first, each with why it wasn't and where all it printed
// is kept.
function checksOf(s: Stage): StepCheck[] {
  const order = { unmet: 0, "can't tell": 1, yours: 1, met: 2 } as const;
  return [...(s.checks ?? [])]
    .sort((a, b) => order[a.result ?? "met"] - order[b.result ?? "met"])
    .map((c) => ({ text: c.text, result: c.result, why: c.evidence?.split("\n")[0], kept: c.kept }));
}

// Stepper is a mission's stages as its Summary has them: a MissionPlan,
// with what's to be known of each step worked out here.
export function Stepper({ stages }: { stages: Stage[] }) {
  return (
    <MissionPlan>
      {stages.map((s) => (
        <MissionPlanStep
          key={s.n}
          n={s.n}
          title={s.title}
          state={s.state}
          hint={s.type ? `${s.type}: ${s.text}` : s.text}
          why={s.why}
          under={s.round && s.state !== "accepted" ? <MissionPlanCheckList checks={checksOf(s)} /> : null}
        >
          {stepNotes(s)}
        </MissionPlanStep>
      ))}
    </MissionPlan>
  );
}

// stepNotes is what's to be known of a step, under its name, or nothing
// where there's nothing to say.
function stepNotes(s: Stage): ReactNode {
  if (
    !(
      s.crew?.length ||
      (s.checkIn && s.state !== "accepted") ||
      (s.who && s.who !== "crew") ||
      s.opensPR ||
      s.checks?.length ||
      s.limit ||
      s.carryOn ||
      s.onlyIf ||
      s.inEach ||
      s.skipped ||
      s.attempts
    )
  )
    return null;
  return (
    <>
      {s.who && s.who !== "crew" && <StepWho s={s} />}
      {s.type === "Verify" && (
        <MissionPlanNote icon="list-checks" title="A verifier's: crew who didn't do the work show it by using it, and hand over a bundle">
          verifier
        </MissionPlanNote>
      )}
      {s.opensPR && s.who !== "ws" && (
        <MissionPlanNote icon="pr" title="It makes a pull request: its crew open it through ws, which holds the guarantees about it">
          opens a pull request
        </MissionPlanNote>
      )}
      {s.skipped ? (
        <MissionPlanNote icon="minus" title={s.why ? `Skipped: ${s.why}` : "Skipped"}>
          skipped
        </MissionPlanNote>
      ) : s.onlyIf ? (
        <MissionPlanNote icon="question" yours={s.held} title={`Only if ${s.onlyIf}: the orchestrator starts it or skips it`}>
          only if
        </MissionPlanNote>
      ) : null}
      {s.inEach ? <StepRuns s={s} /> : null}
      {s.attempts ? (
        <MissionPlanNote icon="rotate" title={`Started again ${s.attempts} ${s.attempts === 1 ? "time" : "times"}`}>
          attempt {s.attempts + 1}
        </MissionPlanNote>
      ) : null}
      {s.carryOn ? (
        <MissionPlanNote icon="reply" title={`The crew who did step ${s.carryOn} carry on with this one`}>
          carries on from {s.carryOn}
        </MissionPlanNote>
      ) : null}
      {s.limit && s.state !== "accepted" && <StepLimit s={s} />}
      {s.checks?.length ? <StepChecks s={s} /> : null}
      {s.crew?.map((c) => (
        <CrewBadge key={c} id={c} />
      ))}
      {s.checkIn && s.state !== "accepted" && (
        <span className="dim" title="The orchestrator stops for you after this stage">
          <Icon name="pause" size={11} /> check in after
        </span>
      )}
    </>
  );
}

// Outputs (a mission's recorded artifacts: its documents, pull requests
// and artifacts): a short title, what the agent said of it under, and its
// kind and state as marks. An artifact (a3) opens on its own page,
// /artifacts/a3.

const kindIcon: Record<string, IconName> = {
  proposal: "file",
  adr: "gavel",
  note: "book",
  doc: "file",
  handoff: "arrow-right",
  review: "eye",
  plan: "layers",
  artifact: "artifact",
};

const kindPlural: Record<string, string> = {
  proposal: "Proposals",
  adr: "Decision records",
  note: "Notes",
  doc: "Documents",
  handoff: "Handoffs",
  review: "Reviews",
  plan: "Plans",
};

export const kindTitle = (kind: string) => kindPlural[kind] ?? capital(kind) + "s";

// artifactTitle is a document's name, short: its label when it's a name,
// else made from its kind and who made it, with the label to describe it.
export function artifactTitle(a: Pick<Artifact, "kind" | "label" | "ref" | "by">): { title: string; desc?: string } {
  const label = a.label?.trim() ?? "";
  const review = /^Review (r\d+) of (.+?): (.+)$/.exec(label);
  if (review) return { title: `Review ${review[1]} of ${review[2]}`, desc: review[3][0].toUpperCase() + review[3].slice(1) };
  if (a.kind === "handoff") return { title: a.by ? `${a.by}'s handoff` : "Handoff", desc: label || undefined };
  if (label && label.length <= 64) return { title: label };
  const base = a.ref.split(/[:/]/).pop() ?? a.ref;
  return { title: label ? `${kindTitle(a.kind).replace(/s$/, "")}${a.by ? ` by ${a.by}` : ""}` : base, desc: label || undefined };
}

const stateTone: Record<string, Tone> = {
  draft: "quiet",
  private: "quiet",
  open: "waiting",
  "in review": "waiting",
  approved: "done",
  landed: "done",
  merged: "done",
  missing: "danger",
  // An artifact's: live, promoted, discarded.
  live: "progress",
  promoted: "done",
  discarded: "quiet",
};

// isArtifact is whether a mission's output is an artifact (a3): opened on
// its own page, with its versions.
export const isArtifact = (a: Pick<Artifact, "kind" | "ref">) => a.kind === "artifact" && /^a[1-9]\d*$/.test(a.ref);

export function ArtifactState({ state }: { state: string }) {
  return (
    <Chip tone={stateTone[state] ?? "quiet"}>
      {state === "private" && <Icon name="lock" size={11} />}
      {capital(state)}
    </Chip>
  );
}

export function ArtifactThreads({ t }: { t: NonNullable<Artifact["threads"]> }) {
  return (
    <Chip tone={t.waiting ? "attention" : "quiet"} title={`${t.open} open threads${t.waiting ? `, ${t.waiting} waiting on you` : ""}`}>
      <Icon name="message" size={11} />
      {t.open}
      {t.waiting > 0 && ` · ${t.waiting} your turn`}
    </Chip>
  );
}

export function ArtifactRow({ a, selected, open, compact, id }: { a: Artifact; selected?: boolean; open: (a: Artifact) => void; compact?: boolean; id?: string }) {
  const { title, desc } = artifactTitle(a);
  const openable = a.readable || !!a.url || isArtifact(a);
  return (
    <div
      id={id}
      className={`artifact-row${selected ? " selected" : ""}${openable ? " openable" : ""}${compact ? " compact" : ""}`}
      onClick={() => openable && open(a)}
      onKeyDown={(e) => e.key === "Enter" && openable && open(a)}
      role={openable ? "link" : undefined}
      tabIndex={openable ? 0 : undefined}
      title={desc}
    >
      <Icon name={kindIcon[a.kind] ?? "file"} size={14} className="artifact-icon" />
      <div className="artifact-body">
        <div className="artifact-line">
          <span className="artifact-title">{title}</span>
          {isArtifact(a) && !!a.versions && (
            <Chip tone="quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
              v{a.versions}
            </Chip>
          )}
          {(!compact || isArtifact(a)) && <ArtifactState state={a.state} />}
          {a.threads && <ArtifactThreads t={a.threads} />}
          {!a.readable && a.url && <Icon name="external" size={12} className="dim" />}
        </div>
        {desc && (
          <div className="artifact-desc">
            <Inline text={desc} lines={compact ? 1 : 2} />
          </div>
        )}
        {compact ? (
          <div className="art-meta">
            {isArtifact(a) ? `${a.ref}${a.by ? ` · ${a.by}` : ""}` : `${a.kind} · ${a.state}`}
          </div>
        ) : (
          <div className="art-meta">
            {a.by && <span>{a.by}</span>}
            {a.added && <Time iso={a.added} as="relative" />}
            <Copyable text={a.ref} title="Copy its ref" />
          </div>
        )}
      </div>
    </div>
  );
}

// shortPath shows a capsule's path from its repo down: the workspace's
// own folders are the same for every capsule.
export const shortPath = (p: string) => p.replace(/^.*\/repos\//, "repos/");

// CapsuleBadges: uncommitted changes, ahead and behind, its PR and checks.
export function CapsuleBadges({ c }: { c: CapsuleDetail }) {
  return (
    <span className="badges">
      {c.landed ? (
        <Chip tone="done" title="Its commits were pushed straight to the default branch: its repo has no pull requests">
          Landed
        </Chip>
      ) : (
        !c.exists && <Chip tone="quiet">Burned</Chip>
      )}
      {c.direct && c.exists && !c.landed && (
        <Chip tone={c.toLand ? "waiting" : "quiet"} title="Its repo has no pull requests: what's committed here lands straight on the default branch, when you say (from its review, or ws land)">
          {c.toLand ? `${c.toLand} to land` : "No pull request"}
        </Chip>
      )}
      {c.dirty && (
        <Chip tone="attention" title="Files changed in the capsule and not committed: reviews and crew after don't see them">
          Uncommitted changes
        </Chip>
      )}
      {!!c.ahead && <Chip tone="quiet" title={`${c.ahead} commits not on the default branch`}>{c.ahead} ahead</Chip>}
      {!!c.behind && <Chip tone="quiet" title={`${c.behind} commits behind the default branch`}>{c.behind} behind</Chip>}
      {c.pr && (
        <a href={c.pr.url} target="_blank" rel="noopener noreferrer" className={chipClass(c.pr.merged ? "done" : "waiting")} onClick={(e) => e.stopPropagation()} title={c.pr.title}>
          <Icon name="pr" size={11} />#{c.pr.number}
          {c.pr.merged ? " merged" : ""}
        </a>
      )}
      {c.pr?.checks && (
        <Chip tone={/fail/i.test(c.pr.checks) ? "danger" : /pass|success/i.test(c.pr.checks) ? "done" : "waiting"}>
          checks {c.pr.checks.toLowerCase()}
        </Chip>
      )}
    </span>
  );
}

// endedWords is how crew ended, in words: stopped by a restart, closed once
// it handed its work back, stopped by you, or its exit's words.
export function endedWords(c: Pick<CrewDetail, "how" | "exit">): string | undefined {
  switch (c.how) {
    case "interrupted":
      return "stopped by a daemon restart";
    case "handed-back":
      return "closed: it had handed its work back";
    case "stopped":
      return "stopped";
  }
  return exitWords(c.exit);
}

// CrewRow is a crew in a line: its colour and id, where it stands, its
// harness and capsule, and what it last said of itself.
export function CrewRow({ c, slug, detail }: { c: CrewDetail; slug: string; detail?: boolean }) {
  const st = crewStatus(c);
  const ended = endedWords(c);
  const body: ReactNode = (
    <>
      <CrewBadge id={c.id} live={!c.ended} />
      <Status s={st} />
      <span className="crew-what">
        {c.role === "briefing" ? "briefing" : c.model ? `${c.harness} on ${c.model}` : c.harness}
        {c.capsule && <span className="dim"> in {c.capsule}</span>}
      </span>
    </>
  );
  return (
    <div className="crew-row">
      <div className="crew-row-line">
        {c.ended ? body : (
          <Link to={`/missions/${slug}/crew/${c.id}`} className="crew-row-link" title={`Open ${c.id}'s terminal`}>
            {body}
          </Link>
        )}
        {c.ended && (
          <Link to={`/missions/${slug}/crew/${c.id}`} className="crew-transcript-link" title={`Read ${c.id}'s transcript`}>
            Transcript
          </Link>
        )}
      </div>
      {c.note && (
        <div className="crew-note">
          <Inline text={c.note} lines={detail ? undefined : 2} />
        </div>
      )}
      {detail && (
        <div className="crew-times">
          Started <Time iso={c.started} />
          {c.endedAt ? (
            <>
              , ended <Time iso={c.endedAt} /> after {duration(c.started, c.endedAt)}
              {ended && <span title={c.exit}>: {ended}</span>}
            </>
          ) : (
            <> · running for {duration(c.started)}{c.status === "done" ? ", session still open" : ""}</>
          )}
        </div>
      )}
      {detail && c.dispatch && (
        <div className="crew-dispatch">
          <span className="dim">Dispatched to </span>
          <Inline text={c.dispatch} lines={3} />
        </div>
      )}
    </div>
  );
}
