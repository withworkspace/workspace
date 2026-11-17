import { createContext, Fragment, type ReactNode, useContext, useState } from "react";
import { Link } from "react-router";
import type { Artifact, CapsuleDetail, CrewDetail, Decision, MissionDetail, MissionEvent, Stage } from "../api";
import { Icon, type IconName } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { HoverCard } from "../ui/hover";
import { Copyable } from "../ui/kit";
import { crewStatus, exitWords, Status } from "../ui/status";
import { dayKey, dayLabel, duration, stamp, Time, useNow } from "../ui/time";

// The pieces a mission's page draws with: who did what (events), stages,
// decisions, documents and capsules.

// crewHue is a crew's own colour within its mission, the same wherever it
// shows: c1 is always the first.
const hues = [228, 152, 32, 330, 192, 272, 96, 8];
export function crewHue(id: string): number {
  const n = Number(/\d+/.exec(id)?.[0] ?? 0);
  return hues[(Math.max(n, 1) - 1) % hues.length];
}

// MissionData is the mission a page is about, for what draws a ref (a crew's
// badge) to say more about it on hover.
export const MissionData = createContext<MissionDetail | undefined>(undefined);

export function CrewBadge({ id, live }: { id: string; live?: boolean }) {
  const d = useContext(MissionData);
  const c = d?.crew.find((x) => x.id === id);
  const badge = (
    <span className={`crew-badge${live ? " live" : ""}`} style={{ "--hue": crewHue(id) } as React.CSSProperties}>
      {id}
    </span>
  );
  if (!c) return badge;
  // A hover card: who it is, where it stands, what it last said.
  const st = crewStatus(c);
  return (
    <HoverCard trigger={badge}>
      <span className="hovercard-head">
        {badge}
        <Status s={st} />
      </span>
      <span className="hovercard-line">
        {c.role === "briefing" ? "Briefing agent" : c.model ? `${c.harness} on ${c.model}` : c.harness}
        {c.capsule && <span className="dim"> in {c.capsule}</span>}
      </span>
      <span className="hovercard-line dim">
        Started <Time iso={c.started} />
        {c.endedAt && (
          <>
            , ended <Time iso={c.endedAt} />
          </>
        )}
      </span>
      {(c.note || c.lastReply) && (
        <span className="hovercard-said">
          <Inline text={c.note || c.lastReply || ""} lines={3} />
        </span>
      )}
      {d && (
        <Link to={`/missions/${d.slug}/crew/${c.id}`} className="hovercard-link">
          {c.ended ? "Read its transcript" : "Open its terminal"} <Icon name="arrow-right" size={11} />
        </Link>
      )}
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
    <span className={`step-who ${s.who === "you" ? "yours" : ""}`} title={title}>
      <Icon name={icon} size={11} /> {label}
    </span>
  );
}

// StepRuns is where a step that runs in several repos runs: the repos
// once they're named, each done or not, or what its words say before.
function StepRuns({ s }: { s: Stage }) {
  const runs = s.runs ?? [];
  if (!runs.length) {
    return (
      <span className="step-who" title={`In each ${s.inEach}: the orchestrator names the repos when it starts`}>
        <Icon name="layers" size={11} /> in each {s.inEach}
      </span>
    );
  }
  const done = runs.filter((r) => r.done).length;
  return (
    <span className="step-who" title={runs.map((r) => `${r.repo}${r.done ? ": done" : ""}`).join("\n")}>
      <Icon name="layers" size={11} /> {runs.length === done ? `in ${runs.length} repos` : `${done} of ${runs.length} repos done`}
    </span>
  );
}

// StepLimit is a Work step's clock: its limit, when it runs out, or that
// it has.
function StepLimit({ s }: { s: Stage }) {
  if (s.outOfTime) {
    return (
      <span className="step-who step-late" title={`Its limit (${s.limit}) ran out while its crew were at it`}>
        <Icon name="clock" size={11} /> out of time
      </span>
    );
  }
  const at = s.deadline ? new Date(s.deadline) : null;
  return (
    <span className="step-who" title={`Its limit: ${s.limit}. ws holds the clock from when crew start on it.`}>
      <Icon name="clock" size={11} />{" "}
      {at ? `until ${at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : (s.limit ?? "").split(/[,;]/)[0]}
    </span>
  );
}

// StepChecks is how many of a Work step's checks are met, or how many it
// has before they've been run.
function StepChecks({ s }: { s: Stage }) {
  const checks = s.checks ?? [];
  const met = checks.filter((c) => c.result === "met").length;
  // A Verify step is held up by what's unmet: what its verifier left for
  // you (can't tell, yours) is yours when you look at its bundle.
  const short = s.type === "Verify" ? checks.some((c) => c.result === "unmet") : met < checks.length;
  const title = checks.map((c) => `${c.result ? `${c.result}: ` : ""}${c.text}`).join("\n");
  if (!s.round) {
    return (
      <span className="step-who" title={`Done when these are true:\n${title}`}>
        <Icon name="list" size={11} /> {checks.length} {checks.length === 1 ? "check" : "checks"}
      </span>
    );
  }
  return (
    <span className={`step-who${short ? " step-late" : ""}`} title={`Round ${s.round}:\n${title}`}>
      <Icon name={short ? "alert" : "check"} size={11} /> {met} of {checks.length} met
    </span>
  );
}

// CheckResults is what a Work step's checks came to in their latest
// round, those not met first.
function CheckResults({ s }: { s: Stage }) {
  const order = { unmet: 0, "can't tell": 1, yours: 1, met: 2 } as const;
  const checks = [...(s.checks ?? [])].sort((a, b) => order[a.result ?? "met"] - order[b.result ?? "met"]);
  return (
    <ul className="step-checks">
      {checks.map((c) => (
        <li key={c.text} className={`step-check ${c.result === "met" ? "met" : c.result ? "not-met" : ""}`}>
          <Icon name={c.result === "met" ? "check" : c.result === "unmet" ? "x" : "question"} size={11} />{" "}
          <span className="step-check-text">
            <Inline text={c.text} />
          </span>
          {c.result && c.result !== "met" && c.evidence && <span className="step-check-why">{c.evidence.split("\n")[0]}</span>}
        </li>
      ))}
    </ul>
  );
}

// CompactMeta is a step in the plan beside a mission: nothing but its name,
// unless it needs you to look: the current step's crew and whether it's
// yours, checks not met, time run out.
function CompactMeta({ s }: { s: Stage }) {
  const current = s.state === "current";
  const unmet = !!s.round && (s.checks ?? []).some((c) => c.result !== "met");
  const yours = current && (s.who === "you" || s.held);
  const late = !!s.outOfTime && s.state !== "accepted";
  if (!(current && s.crew?.length) && !yours && !unmet && !late) return null;
  return (
    <span className="step-meta">
      {yours && (
        <span className="step-who yours" title="Yours: only you pass it">
          <Icon name="user" size={11} /> yours
        </span>
      )}
      {late && <StepLimit s={s} />}
      {unmet && <StepChecks s={s} />}
      {current && s.crew?.map((c) => <CrewBadge key={c} id={c} />)}
    </span>
  );
}

export function Stepper({ stages, compact }: { stages: Stage[]; compact?: boolean }) {
  return (
    <ol className={`stepper${compact ? " compact" : ""}`}>
      {stages.map((s) => (
        <li key={s.n} className={`step ${s.state}`} title={s.type ? `${s.type}: ${s.text}` : s.text}>
          <span className="step-mark" aria-hidden>
            {s.state === "accepted" ? <Icon name="check" size={10} /> : null}
          </span>
          <span className="step-body">
            <span className="step-title">
              <span className="step-n">{s.n}</span> {s.title}
              <span className="sr-only">{s.state === "accepted" ? " (done)" : s.state === "current" ? " (now)" : ""}</span>
            </span>
            {compact ? (
              <CompactMeta s={s} />
            ) : (s.crew?.length ||
              (s.checkIn && s.state !== "accepted") ||
              (s.who && s.who !== "crew") ||
              s.opensPR ||
              s.checks?.length ||
              s.limit ||
              s.carryOn ||
              s.onlyIf ||
              s.inEach ||
              s.skipped ||
              s.attempts) && (
              <span className="step-meta">
                {s.who && s.who !== "crew" && <StepWho s={s} />}
                {s.type === "Verify" && (
                  <span className="step-who" title="A verifier's: crew who didn't do the work show it by using it, and hand over a bundle">
                    <Icon name="list-checks" size={11} /> verifier
                  </span>
                )}
                {s.opensPR && s.who !== "ws" && (
                  <span className="step-who" title="It makes a pull request: its crew open it through ws, which holds the guarantees about it">
                    <Icon name="pr" size={11} /> opens a pull request
                  </span>
                )}
                {s.skipped ? (
                  <span className="step-who" title={s.why ? `Skipped: ${s.why}` : "Skipped"}>
                    <Icon name="minus" size={11} /> skipped
                  </span>
                ) : s.onlyIf ? (
                  <span className={`step-who${s.held ? " yours" : ""}`} title={`Only if ${s.onlyIf}: the orchestrator starts it or skips it`}>
                    <Icon name="question" size={11} /> only if
                  </span>
                ) : null}
                {s.inEach ? <StepRuns s={s} /> : null}
                {s.attempts ? (
                  <span className="step-who" title={`Started again ${s.attempts} ${s.attempts === 1 ? "time" : "times"}`}>
                    <Icon name="rotate" size={11} /> attempt {s.attempts + 1}
                  </span>
                ) : null}
                {s.carryOn ? (
                  <span className="step-who" title={`The crew who did step ${s.carryOn} carry on with this one`}>
                    <Icon name="reply" size={11} /> carries on from {s.carryOn}
                  </span>
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
              </span>
            )}
            {s.why && !compact && <span className="step-why">{s.why}</span>}
            {!compact && s.round && s.state !== "accepted" ? <CheckResults s={s} /> : null}
          </span>
        </li>
      ))}
    </ol>
  );
}

// Decisions: those that stand, newest first; the rest (replaced or
// withdrawn) folded.
export function Decisions({ list, limit }: { list: Decision[]; limit?: number }) {
  const [all, setAll] = useState(false);
  const current = list.filter((d) => d.state === "current");
  const shown = limit && !all ? current.slice(0, limit) : current;
  const old = list.filter((d) => d.state !== "current");
  const more = current.length - shown.length;
  return (
    <div className="decisions">
      {shown.map((d) => (
        <div key={d.id} className="decision">
          <div className="decision-what">
            <Inline text={d.what} />
          </div>
          {d.why && (
            <div className="decision-why">
              <Inline text={d.why} />
            </div>
          )}
          <div className="decision-meta">
            <Copyable text={d.id} className="ref" />
            <span>{d.by === "you" ? "You" : d.by}</span>
            {d.from && <span>from {d.from}</span>}
            <Time iso={d.at} as="relative" />
          </div>
        </div>
      ))}
      {(more > 0 || (old.length > 0 && (all || !limit))) && (
        <div className="decisions-more">
          {more > 0 && (
            <button className="ghost small" onClick={() => setAll(true)}>
              Show {more} more
            </button>
          )}
          {old.length > 0 && (all || !limit) && (
            <details className="old-decisions">
              <summary>{old.length} replaced or withdrawn</summary>
              {old.map((d) => (
                <div key={d.id} className="decision old">
                  <div className="decision-what">
                    <Inline text={d.what} />
                  </div>
                  <div className="decision-meta">
                    <Copyable text={d.id} className="ref" />
                    <span>{d.state === "superseded" ? `Replaced by ${d.supersededBy}` : `Withdrawn: ${d.withdrawnWhy}`}</span>
                  </div>
                </div>
              ))}
            </details>
          )}
        </div>
      )}
    </div>
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

export const kindTitle = (kind: string) => kindPlural[kind] ?? kind[0].toUpperCase() + kind.slice(1) + "s";

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

const stateTone: Record<string, string> = {
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
    <span className={`chip tone-${stateTone[state] ?? "quiet"}`}>
      {state === "private" && <Icon name="lock" size={11} />}
      {state[0]?.toUpperCase() + state.slice(1)}
    </span>
  );
}

export function ArtifactThreads({ t }: { t: NonNullable<Artifact["threads"]> }) {
  return (
    <span className={`chip ${t.waiting ? "tone-attention" : "tone-quiet"}`} title={`${t.open} open threads${t.waiting ? `, ${t.waiting} waiting on you` : ""}`}>
      <Icon name="message" size={11} />
      {t.open}
      {t.waiting > 0 && ` · ${t.waiting} your turn`}
    </span>
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
            <span className="chip tone-quiet" title={`${a.versions} ${a.versions === 1 ? "version" : "versions"}`}>
              v{a.versions}
            </span>
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
            <Copyable text={a.ref} className="ref" title="Copy its ref" />
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
        <span className="chip tone-done" title="Its commits were pushed straight to the default branch: its repo has no pull requests">
          Landed
        </span>
      ) : (
        !c.exists && <span className="chip tone-quiet">Burned</span>
      )}
      {c.direct && c.exists && !c.landed && (
        <span className={`chip ${c.toLand ? "tone-waiting" : "tone-quiet"}`} title="Its repo has no pull requests: what's committed here lands straight on the default branch, when you say (from its review, or ws land)">
          {c.toLand ? `${c.toLand} to land` : "No pull request"}
        </span>
      )}
      {c.dirty && (
        <span className="chip tone-attention" title="Files changed in the capsule and not committed: reviews and crew after don't see them">
          Uncommitted changes
        </span>
      )}
      {!!c.ahead && <span className="chip tone-quiet" title={`${c.ahead} commits not on the default branch`}>{c.ahead} ahead</span>}
      {!!c.behind && <span className="chip tone-quiet" title={`${c.behind} commits behind the default branch`}>{c.behind} behind</span>}
      {c.pr && (
        <a
          href={c.pr.url}
          target="_blank"
          rel="noopener noreferrer"
          className={`chip ${c.pr.merged ? "tone-done" : "tone-waiting"}`}
          onClick={(e) => e.stopPropagation()}
          title={c.pr.title}
        >
          <Icon name="pr" size={11} />#{c.pr.number}
          {c.pr.merged ? " merged" : ""}
        </a>
      )}
      {c.pr?.checks && (
        <span className={`chip ${/fail/i.test(c.pr.checks) ? "tone-danger" : /pass|success/i.test(c.pr.checks) ? "tone-done" : "tone-waiting"}`}>
          checks {c.pr.checks.toLowerCase()}
        </span>
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

// Timeline is the mission's life in a strip: when each crew worked (in its
// colour), and the moments that mattered (stages accepted, reviews,
// decisions, what you said), from its start to now.
export function Timeline({ d }: { d: MissionDetail }) {
  const now = useNow();
  const start = new Date(d.created).getTime();
  const lastEvent = d.events.length ? new Date(d.events[d.events.length - 1].time).getTime() : start;
  const end = d.state === "archived" || d.state === "landed" ? Math.max(lastEvent, start + 60_000) : now;
  const span = Math.max(end - start, 60_000);
  const at = (iso: string | number) => Math.min(100, Math.max(0, ((new Date(iso).getTime() - start) / span) * 100));
  // When each crew ran: from the log, start (or resume) to end, so crew
  // resumed days later show as two runs, not one long one; crew with
  // nothing in the log, by when they started and ended.
  const spans: { c: (typeof d.crew)[number]; from: number; to: number }[] = [];
  for (const c of d.crew) {
    const mine = d.events.filter((e) => e.by === c.id && /^crew_(started|resumed|ended)$/.test(e.kind));
    if (!mine.length) {
      const from = new Date(c.started).getTime();
      spans.push({ c, from, to: c.endedAt ? new Date(c.endedAt).getTime() : c.ended ? from : now });
      continue;
    }
    let open: number | undefined;
    for (const e of mine) {
      const t = new Date(e.time).getTime();
      if (e.kind === "crew_ended") {
        spans.push({ c, from: open ?? t, to: t });
        open = undefined;
      } else open ??= t;
    }
    if (open !== undefined) spans.push({ c, from: open, to: c.ended ? open : now });
  }
  // In lanes, so crew running at once don't cover each other.
  const lanes: number[] = [];
  const bars = spans
    .sort((a, b) => a.from - b.from)
    .map((sp, i) => {
      let lane = lanes.findIndex((free) => free <= sp.from);
      if (lane < 0) lane = lanes.push(0) - 1;
      lanes[lane] = sp.to;
      return { ...sp, lane, key: `${sp.c.id}-${i}` };
    });
  const marks = d.events.flatMap((e): { e: MissionEvent; icon: IconName; cls: string }[] => {
    if (e.kind === "stage" && /accepted/.test(e.text)) return [{ e, icon: "check", cls: "done" }];
    if (e.kind === "review") return [{ e, icon: "eye", cls: "you" }];
    if (e.kind === "decision") return [{ e, icon: "gavel", cls: "decided" }];
    if (e.kind === "flag" || e.kind === "crew_asked") return [{ e, icon: "question", cls: "ask" }];
    return [];
  });
  if (!bars.length && !marks.length) return null;
  const laneCount = Math.max(1, Math.min(lanes.length, 3));
  return (
    <div className="timeline" style={{ "--lanes": laneCount } as React.CSSProperties} aria-label="The mission over time">
      <div className="timeline-track">
        {bars
          .filter((b) => b.lane < 3)
          .map((b) => (
            <span
              key={b.key}
              className={`timeline-bar${b.c.ended || b.to < now - 60_000 ? "" : " live"}`}
              style={
                {
                  "--hue": crewHue(b.c.id),
                  left: `${at(b.from)}%`,
                  width: `max(3px, ${at(b.to) - at(b.from)}%)`,
                  top: `${b.lane * 7}px`,
                } as React.CSSProperties
              }
              title={`${b.c.id}: ${stamp(b.from)}${b.to < now - 60_000 ? ` to ${stamp(b.to)} (${duration(b.from, b.to)})` : ", running now"}`}
            />
          ))}
        {marks.map((m, i) => (
          <span key={i} className={`timeline-mark ${m.cls}`} style={{ left: `${at(m.e.time)}%` }} title={`${stamp(m.e.time)}: ${m.e.text}`}>
            <Icon name={m.icon} size={9} />
          </span>
        ))}
      </div>
      <div className="timeline-ends">
        <Time iso={d.created} />
        <span>{d.state === "archived" || d.state === "landed" ? <Time iso={new Date(end).toISOString()} /> : "now"}</span>
      </div>
    </div>
  );
}
