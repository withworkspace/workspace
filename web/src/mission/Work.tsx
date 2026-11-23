import { useCallback, useMemo, useState } from "react";
import { Link } from "react-router";
import { type Artifact, type CapsuleDetail, type ChangedFile, type CrewDetail, type MissionDetail, type Stage, capsuleAPI, getArtifact, getRepos, getSummary } from "../api";
import { type Reload, useLoaded } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Empty, ErrorCard, Skeleton } from "../ui/kit";
import { Copyable } from "../ui/Copyable";
import { Keys } from "../ui/Keys";
import { crewStatus, Status } from "../ui/status";
import { Time } from "../ui/time";
import { CapsuleEnds, EndedCapsules } from "./Ending";
import { canOpenPR, OpenPR } from "./OpenPR";
import { RunsHere } from "./Runs";
import { ArtifactState, ArtifactThreads, artifactTitle, CrewBadge, endedWords, isArtifact, kindTitle, shortPath } from "./parts";
import { type Home, type Work as WorkModel, type WorkCapsule, type WorkState, fileGroups, outputHome, outputKey } from "./workRows";
import { Chip, chipClass } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button, buttonClass } from "../ui/Button";
import { GroupHead } from "../ui/GroupHead";
import { Input } from "../ui/Input";
import { usePref } from "../prefs";

// The mission's Work tab: what's been made, a row each, and the one that's
// open beside it. Capsules come first, in the order they need you (in
// hand, up next, done, which folds to a line), then its outputs. A capsule
// opens on who's been in it and what they left, the plan's steps that ran
// there, its commits, and the files that differ from where it left the
// default branch; an output opens to be read. On a wide window the tab
// fills what's under the mission's bar and each column scrolls by itself
// (.mission-page.fills); on a narrow one they stack, and the page scrolls.

// useRepoLabel is a repo's name as the workspace calls it (ws, for
// workspace), and its own until that's known.
export function useRepoLabel(): (repo: string) => string {
  const { value: repos } = useLoaded(getRepos, [], { keep: true });
  return useCallback((repo: string) => repos?.find((r) => r.name === repo)?.label ?? repo, [repos]);
}

const bundleLink = (slug: string, id: string) => `/missions/${slug}?${new URLSearchParams({ tab: "verify", b: id })}`;
const count = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? "" : "s"}`;

// useSummary is a capsule in brief, read again when it moves on (its head,
// or whether it's all committed), when the daemon has read it again and
// found it changed (seen: the workspace's readsSeen), and for one with
// changes not committed, whenever fresh changes: what's there is still
// being written. The daemon answers as it last read it, and reads it again
// behind the answer. kept asks only what it has: a row doesn't have a
// capsule read, only the capsule that's shown does. Unread, it's nothing
// yet.
function useSummary(slug: string, c: CapsuleDetail, o: { seen?: string; fresh?: unknown; kept?: boolean }) {
  return useLoaded(
    c.exists ? () => getSummary(capsuleAPI(slug, c.repo, c.name), o.kept).then((s) => (s.unread ? undefined : s)) : null,
    [slug, c.repo, c.name, c.exists, c.head, c.dirty, o.seen, o.kept, c.dirty ? o.fresh : undefined],
    { keep: true },
  );
}

function Lines({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span className="work-lines">
      {additions > 0 && <span className="plus">+{additions.toLocaleString()}</span>}
      {additions > 0 && deletions > 0 && " "}
      {deletions > 0 && <span className="minus">−{deletions.toLocaleString()}</span>}
    </span>
  );
}

function StateChip({ s }: { s: WorkState }) {
  const body = (
    <>
      {s.live ? <span className="work-live-dot" /> : s.icon && <Icon name={s.icon} size={11} />}
      {s.text}
    </>
  );
  const cls = chipClass(s.tone ?? "quiet");
  return s.url ? (
    <a href={s.url} target="_blank" rel="noopener noreferrer" className={cls} onClick={(e) => e.stopPropagation()}>
      {body}
    </a>
  ) : (
    <span className={cls}>{body}</span>
  );
}

export function Work({
  d,
  w,
  shown,
  select,
  review,
  openArtifact,
  reload,
  seen,
}: {
  d: MissionDetail;
  w: WorkModel;
  shown?: string; // the row that's open: a capsule's key or an output's
  select: (key: string) => void;
  review: (c: CapsuleDetail) => void;
  openArtifact: (a: Artifact) => void;
  reload: Reload;
  // seen is the workspace's readsSeen: a capsule's summary read again.
  seen?: string;
}) {
  const label = useRepoLabel();
  const [doneOpen, setDoneOpen] = usePref("workDone");
  const capsule = w.capsules.find((x) => x.key === shown);
  const output = capsule ? undefined : w.outputs.find((a) => outputKey(a) === shown);
  // Done is open while what's shown is in it.
  const showDone = doneOpen || capsule?.where === "done";
  const toggleDone = () => {
    const open = !showDone;
    setDoneOpen(open);
    if (!open && capsule?.where === "done" && w.opensOn && w.opensOn !== capsule.key) select(w.opensOn);
  };
  const rows = (list: WorkCapsule[]) => list.map((x) => <CapsuleRow key={x.key} d={d} x={x} selected={x.key === shown} select={select} label={label} review={review} seen={seen} />);

  if (!w.capsules.length && !w.outputs.length && !d.endedCapsules?.length) {
    return (
      <Empty icon="box" title="Nothing's been made yet">
        A capsule is a worktree the mission's crew work in. Crew lift one when they start; <code>ws lift &lt;repo&gt; &lt;name&gt;</code> lifts one by hand. Proposals, notes, handoffs and
        artifacts show here as crew make them.
      </Empty>
    );
  }
  return (
    <div className="work">
      <div className="work-list">
        <GroupHead className="work-group" count={w.capsules.length}>
          Capsules
        </GroupHead>
        {w.hand.length > 0 && <div className="work-sub">In hand</div>}
        {rows(w.hand)}
        {w.next.length > 0 && <div className="work-sub">Up next</div>}
        {rows(w.next)}
        {w.done.length > 0 && (
          <button className="work-sub" aria-expanded={showDone} onClick={toggleDone}>
            <Icon name="chevron-right" size={11} />
            Done
            {!showDone && <span className="work-sub-rest">· {doneWords(w.done, label, d.slug)}</span>}
          </button>
        )}
        {showDone && rows(w.done)}
        {!w.capsules.length && <p className="work-none">None yet: crew lift one when they start.</p>}
        {d.endedCapsules?.length ? (
          <div className="ended-alone">
            <h3>Ended alone</h3>
            <EndedCapsules list={d.endedCapsules} />
          </div>
        ) : null}
        <GroupHead className="work-group" count={w.outputs.length}>
          Outputs
        </GroupHead>
        {w.outputs.map((a) => (
          <OutputRow key={a.ref} a={a} selected={outputKey(a) === shown} select={select} open={openArtifact} />
        ))}
        {!w.outputs.length && <p className="work-none">Nothing yet: proposals, notes, handoffs and artifacts show here as crew make them.</p>}
      </div>
      <section className="work-pane" aria-live="polite">
        {capsule && <CapsulePane key={capsule.key} d={d} x={capsule} label={label} select={select} review={review} reload={reload} seen={seen} />}
        {output && <OutputPane key={output.ref} d={d} a={output} home={outputHome(d, w, output)} label={label} select={select} open={openArtifact} />}
      </section>
    </div>
  );
}

// workWords is what Summary says of the Work tab in a line: what's up
// next and done, and how many outputs.
export function workWords(w: WorkModel, label: (repo: string) => string, slug: string): string {
  const words = [
    w.next.length > 0 && `up next: ${doneWords(w.next, label, slug)}`,
    w.done.length > 0 && `done: ${doneWords(w.done, label, slug)}`,
    w.outputs.length > 0 && `${w.outputs.length} ${w.outputs.length === 1 ? "output" : "outputs"}`,
  ].filter(Boolean);
  return words.join(" · ").replace(/^./, (c) => c.toUpperCase());
}

// doneWords is capsules in a line, as their rows name them (the repo, and
// the capsule's own name when it isn't the mission's), each once, with how
// many when it's more than one.
function doneWords(done: WorkCapsule[], label: (repo: string) => string, slug: string): string {
  const n = new Map<string, number>();
  for (const x of done) {
    const name = label(x.c.repo) + (x.c.name === slug ? "" : ` ${x.c.name}`);
    n.set(name, (n.get(name) ?? 0) + 1);
  }
  return [...n].map(([name, k]) => (k > 1 ? `${name} ×${k}` : name)).join(", ");
}

// A capsule in a line: its repo (and its own name, when that isn't the
// mission's), how much has changed, who's in it now, and where it stands.
function CapsuleRow({ d, x, selected, select, label, review, seen }: { d: MissionDetail; x: WorkCapsule; selected: boolean; select: (key: string) => void; label: (repo: string) => string; review: (c: CapsuleDetail) => void; seen?: string }) {
  const { c } = x;
  // What's known of it: it's read when it's the capsule shown.
  const { value: sum } = useSummary(d.slug, c, { seen, kept: !selected });
  const loose = sum?.changed.filter((f) => f.uncommitted).length ?? 0;
  // A crew who's at work is in the chip already.
  const here = x.live && !x.state.live ? x.live : undefined;
  return (
    <div
      id={`item-${x.key}`}
      className={`work-row${selected ? " selected" : ""}${x.where !== "hand" ? " quiet" : ""}`}
      role="button"
      tabIndex={0}
      aria-current={selected}
      title={`${x.id}${c.branch && c.branch !== c.name ? ` (${c.branch})` : ""}`}
      onClick={() => select(x.key)}
      onDoubleClick={() => review(c)}
      onKeyDown={(e) => e.key === "Enter" && (e.stopPropagation(), select(x.key))}
    >
      <Icon name="box" size={14} />
      <span className="work-row-title">
        <span className="work-row-name">{label(c.repo)}</span>
        {c.name !== d.slug && <span className="mono work-row-slug">{c.name}</span>}
        {sum && (sum.commitCount > 0 || loose > 0) && (
          <span className="work-row-stat">{sum.commitCount > 0 ? <Lines additions={sum.additions} deletions={sum.deletions} /> : `${loose} not committed`}</span>
        )}
      </span>
      <span className="work-row-end">
        {here && <CrewBadge id={here.id} live />}
        <StateChip s={x.state} />
      </span>
    </div>
  );
}

// An output in a line: who left it (a handoff's crew) or what it is, what
// it says, and how it stands or when it was made.
function OutputRow({ a, selected, select, open, made }: { a: Artifact; selected: boolean; select: (key: string) => void; open: (a: Artifact) => void; made?: boolean }) {
  const { title, desc } = artifactTitle(a);
  const key = outputKey(a);
  const handoff = a.kind === "handoff";
  const stands = isArtifact(a) || (a.state !== "private" && a.state !== "draft");
  return (
    <div
      id={made ? undefined : `item-${key}`}
      className={`work-row output${selected ? " selected" : ""}${handoff ? "" : " named"}`}
      role="button"
      tabIndex={0}
      aria-current={selected}
      title={desc ?? title}
      onClick={() => select(key)}
      onDoubleClick={() => open(a)}
      onKeyDown={(e) => e.key === "Enter" && (e.stopPropagation(), select(key))}
    >
      {handoff && a.by ? <CrewBadge id={a.by} /> : <Icon name={handoff ? "arrow-right" : "file"} size={14} />}
      <span className="work-row-title">
        <span className="work-row-name">{handoff ? (desc ?? title) : title}</span>
      </span>
      <span className="work-row-end">
        {a.threads && <ArtifactThreads t={a.threads} />}
        {stands ? <ArtifactState state={a.state} /> : <Time iso={a.added} as="relative" className="work-row-when" />}
      </span>
    </div>
  );
}

// The open capsule: its head (what it is, where it stands, what you can do
// with it), then who's been in it and the steps that ran there, its
// commits, and its changed files beside them.
function CapsulePane({ d, x, label, select, review, reload, seen }: { d: MissionDetail; x: WorkCapsule; label: (repo: string) => string; select: (key: string) => void; review: (c: CapsuleDetail) => void; reload: Reload; seen?: string }) {
  const { c } = x;
  // Read again as the mission moves, while what's there isn't committed.
  const { value: sum, error } = useSummary(d.slug, c, { seen, fresh: d.events.length });
  const open = !d.ending && d.state !== "archived";
  // Landed or merged, its work is the default branch's: there's nothing of
  // it left to hand on or drop.
  const settled = !!c.landed || !!c.pr?.merged;
  const files = sum?.changed ?? [];
  const loose = files.filter((f) => f.uncommitted).length;
  // A handoff is on its crew's row already.
  const crewIds = new Set(x.crew.map((k) => k.id));
  const made = x.made.filter((a) => !(a.kind === "handoff" && a.by && crewIds.has(a.by)));
  const handoff = (k: CrewDetail) => x.made.find((a) => a.ref === k.handoff);
  const quiet = !files.length && !x.crew.length && !x.steps.some((s) => s.state !== "todo");
  return (
    <>
      <header className="work-head">
        <div className="work-head-top">
          <Icon name="box" size={16} />
          <h2>{label(c.repo)}</h2>
          <Copyable text={x.id} className="work-head-slug" title="Copy the capsule" />
          {c.branch && c.branch !== c.name && (
            <Chip tone="quiet" title="Its branch">
              <Icon name="branch" size={11} />
              {c.branch}
            </Chip>
          )}
          <StateChip s={x.state} />
          <div className="work-head-actions">
            {open && canOpenPR(d, c) && <OpenPR d={d} c={c} reload={reload} />}
            {c.exists && (
              <Link to={`/missions/${d.slug}/files/${c.repo}/${c.name}`} className={buttonClass({ size: "small" })} title="Browse its files (f)">
                <Icon name="file" size={12} /> Files <Keys k="f" />
              </Link>
            )}
            {c.exists && (!sum || sum.commitCount > 0) && (
              <Button size="small" onClick={() => review(c)} title="Review its changes (v)">
                <Icon name="eye" /> Review <Keys k="v" />
              </Button>
            )}
            {open && !settled && <CapsuleEnds slug={d.slug} c={c} reload={reload} />}
          </div>
        </div>
        <div className="work-head-meta">
          {c.landed && (
            <span>
              Landed straight on the default branch <Time iso={c.landed} />, and burned
            </span>
          )}
          {!c.exists && !c.landed && <span>Burned: its worktree is gone</span>}
          {sum &&
            (sum.base ? (
              <>
                <span>{sum.commitCount ? `${count(sum.commitCount, "commit")} since it left ${sum.baseRef.replace(/^origin\//, "")}` : "No commits yet"}</span>
                {sum.files > 0 && (
                  <span>
                    {count(sum.files, "file")} <Lines additions={sum.additions} deletions={sum.deletions} />
                  </span>
                )}
                <span className={loose ? "tone-attention work-loose" : undefined} title={loose ? "Changed in the capsule and not committed: reviews and crew after don't see them" : undefined}>
                  {loose ? `${count(loose, "file")} not committed` : "nothing uncommitted"}
                </span>
              </>
            ) : (
              <span>Can't tell what's changed: {sum.baseRef} isn't fetched here.</span>
            ))}
          {!!c.behind && <span title={`${c.behind} commits behind the default branch`}>{c.behind} behind</span>}
          {c.exists && (
            <Copyable text={c.path} title="Copy its folder">
              {shortPath(c.path)}
            </Copyable>
          )}
        </div>
        {c.pr && <div className="work-head-pr">{c.pr.title}</div>}
      </header>
      <div className={`work-body${files.length ? "" : " one"}`}>
        <div className="work-col">
          {x.now && <Now d={d} x={x} review={review} />}
          {error && !sum && <ErrorCard title="Couldn't read what's changed" error={error} />}
          {x.crew.length > 0 && (
            <>
              <h3>
                Crew here <Count>{x.crew.length}</Count>
              </h3>
              <div className="work-crew">
                {x.crew.map((k) => (
                  <CrewLine key={k.id} d={d} k={k} steps={x.steps.filter((s) => s.crew?.includes(k.id))} handoff={handoff(k)} select={select} />
                ))}
              </div>
            </>
          )}
          <RunsHere slug={d.slug} c={c} open={open} reload={reload} />
          {x.steps.length > 0 && (
            <>
              <h3>
                Steps here <Count>{x.steps.length}</Count>
              </h3>
              <ol className="work-steps">
                {x.steps.map((s) => (
                  <StepLine key={s.n} d={d} x={x} s={s} />
                ))}
              </ol>
            </>
          )}
          {sum && sum.commits.length > 0 && (
            <>
              <h3>
                Commits <Count>{sum.commitCount}</Count>
              </h3>
              <ul className="work-commits">
                {sum.commits.map((m) => (
                  <li key={m.sha}>
                    <Copyable text={m.sha.slice(0, 7)} className="work-commit-sha" title="Copy the commit" />
                    <span className="work-commit-subject" title={m.subject}>
                      {m.subject}
                    </span>
                    <Time iso={m.time} as="relative" className="dim" />
                  </li>
                ))}
                {sum.commitCount > sum.commits.length && <li className="dim">and {sum.commitCount - sum.commits.length} more</li>}
              </ul>
            </>
          )}
          {made.length > 0 && (
            <>
              <h3>
                Made here <Count>{made.length}</Count>
              </h3>
              <div className="work-made">
                {made.map((a) => (
                  <OutputRow key={a.ref} a={a} selected={false} select={select} open={() => select(outputKey(a))} made />
                ))}
              </div>
            </>
          )}
          {quiet && c.exists && (
            <Empty icon="box" title="Nothing's changed here yet">
              {sum?.base ? `It's level with ${sum.baseRef.replace(/^origin\//, "")}` : "It's as it was lifted"}
              {c.head ? ` at ${c.head.slice(0, 7)}` : ""}.{x.steps[0] ? ` Step ${x.steps[0].n}, ${x.steps[0].title}, is the first in it.` : ""}
            </Empty>
          )}
          {!sum && !error && c.exists && <Skeleton rows={3} />}
        </div>
        {files.length > 0 && <FilesCol d={d} c={c} files={files} more={sum?.changedMore} committed={sum?.commitCount ?? 0} />}
      </div>
    </>
  );
}

// Now is the step the mission's on, when it's in this capsule: whose it
// is, in a line, and what moves it on.
function Now({ d, x, review }: { d: MissionDetail; x: WorkCapsule; review: (c: CapsuleDetail) => void }) {
  const s = x.now!;
  const live = x.live && s.crew?.includes(x.live.id) ? x.live : undefined;
  // The bundle a step of yours looks at.
  const bundle = x.bundles.find((b) => b.asked && !b.verdict && !b.replacedBy);
  const tone = s.who === "you" || (s.opensPR && !x.c.pr) ? "attention" : "progress";
  return (
    <div className={`work-now tone-${tone}`}>
      <p>
        <strong>
          Step {s.n} · {s.title}
        </strong>{" "}
        {s.who === "you" ? "is yours." : live ? <>has {live.id} on it, since <Time iso={live.started} as="clock" />.</> : s.who === "ws" ? "is ws's." : s.who === "crew" ? "waits for crew." : `is ${s.whoSays ?? s.who ?? "next"}.`}
      </p>
      {s.who === "you" && s.type === "Review" && x.c.exists && (
        <Button variant="primary" size="small" onClick={() => review(x.c)}>
          <Icon name="eye" /> Review the change
        </Button>
      )}
      {bundle && (
        <Link to={bundleLink(d.slug, bundle.id)} className={buttonClass({ size: "small" })}>
          <Icon name="list-checks" size={12} /> {bundle.id} in Verify
        </Link>
      )}
      {live && (
        <Link to={`/missions/${d.slug}/crew/${live.id}`} className={buttonClass({ size: "small" })}>
          <Icon name="terminal" size={12} /> Watch {live.id}
        </Link>
      )}
    </div>
  );
}

// howEnded is how a crew ended, in a word or two: the whole of it is said
// on hover.
function howEnded(k: CrewDetail): string {
  switch (k.how) {
    case "handed-back":
      return "Handed back";
    case "interrupted":
      return "Stopped by a restart";
    case "stopped":
      return "Stopped";
  }
  return k.status === "done" ? "Done" : "Ended";
}

// A crew who's been in the capsule: where it stands or how it ended, the
// steps it took and when, and what it left.
function CrewLine({ d, k, steps, handoff, select }: { d: MissionDetail; k: CrewDetail; steps: Stage[]; handoff?: Artifact; select: (key: string) => void }) {
  const ended = endedWords(k);
  return (
    <div className="work-crew-row">
      <div className="work-crew-line">
        <CrewBadge id={k.id} live={!k.ended} />
        <span className="work-crew-say">
          {k.ended ? <strong title={ended}>{howEnded(k)}</strong> : <Status s={crewStatus(k)} />}{" "}
          <span className="dim">
            {steps.map((s) => `step ${s.n}, ${s.title}`).join("; ")}
            {steps.length > 0 && " · "}
            <Time iso={k.started} />
            {k.endedAt ? (
              <>
                {" "}
                to <Time iso={k.endedAt} as="clock" />
              </>
            ) : (
              ", still here"
            )}
          </span>
        </span>
        <Link to={`/missions/${d.slug}/crew/${k.id}`} className={buttonClass({ size: "small" }, "crew-line-link")} title={k.ended ? `Read ${k.id}'s transcript` : `Open ${k.id}'s terminal`}>
          {k.ended ? (
            "Transcript"
          ) : (
            <>
              <Icon name="terminal" size={12} /> Terminal
            </>
          )}
        </Link>
      </div>
      {handoff ? (
        <button className="work-left" onClick={() => select(outputKey(handoff))} title={`Read ${k.id}'s handoff`}>
          <Icon name="arrow-right" size={12} />
          <span>
            <Inline text={handoff.label ?? k.note ?? "Its handoff"} lines={2} />
          </span>
        </button>
      ) : (
        k.note && (
          <div className="work-left said">
            <span>
              <Inline text={k.note} lines={2} />
            </span>
          </div>
        )
      )}
    </div>
  );
}

// A step that ran in the capsule, in a line: done, now or to come, who's
// on it, and the bundles that show it. How it passed is said on hover.
function StepLine({ d, x, s }: { d: MissionDetail; x: WorkCapsule; s: Stage }) {
  const bundles = x.bundles.filter((b) => b.step === s.n && !b.replacedBy);
  return (
    <li className={`work-step ${s.state}`} title={s.why ? `${s.title}: ${s.why}` : s.text}>
      <span className="work-step-mark" aria-hidden>
        {s.state === "accepted" && <Icon name="check" size={10} />}
      </span>
      <span className="work-step-n">{s.n}</span>
      <span className="work-step-line">
        <span className="work-step-title">{s.title}</span>
        {s.crew?.length ? s.crew.map((id) => <CrewBadge key={id} id={id} live={d.crew.some((k) => k.id === id && !k.ended)} />) : s.who && s.who !== "crew" && <Chip tone="quiet">{s.who}</Chip>}
        {s.type && s.type !== s.title && <span className="dim">{s.type}</span>}
        {s.state === "current" && <Chip tone="info">now</Chip>}
        {bundles.map((b) => (
          <Link key={b.id} to={bundleLink(d.slug, b.id)} className="work-step-bundle" title={`${b.id}${b.verdict ? `: ${b.verdict}` : ""}${b.stale ? ", of a commit the capsule has moved on from" : ""}`}>
            <Icon name="list-checks" size={12} />
            {b.id}
          </Link>
        ))}
        <span className="sr-only">{s.state === "accepted" ? " (done)" : s.state === "current" ? " (now)" : ""}</span>
      </span>
    </li>
  );
}

const letters: Record<string, string> = { added: "A", deleted: "D", modified: "M", renamed: "R", copied: "C", type: "T" };

// The files that differ from where the capsule left the default branch,
// by top-level folder: a click opens the file in the capsule, with what's
// changed in it marked. Those not committed yet say so; ones to skim are
// quieter.
function FilesCol({ d, c, files, more, committed }: { d: MissionDetail; c: CapsuleDetail; files: ChangedFile[]; more?: number; committed: number }) {
  const [filter, setFilter] = useState("");
  const [shut, setShut] = useState<Set<string>>(() => new Set());
  const groups = useMemo(() => fileGroups(files, filter), [files, filter]);
  const toggle = (dir: string) =>
    setShut((cur) => {
      const next = new Set(cur);
      if (!next.delete(dir)) next.add(dir);
      return next;
    });
  return (
    <div className="work-col work-files">
      <h3>
        {committed ? "Changed files" : "Not committed"} <Count>{files.length + (more ?? 0)}</Count>
        <span className="spacer" />
        {files.length > 8 && <Input type="text" className="work-filter" placeholder="Filter" aria-label="Filter the changed files" value={filter} onChange={(e) => setFilter(e.target.value)} />}
      </h3>
      {groups.map((g) => {
        const open = !shut.has(g.dir) || !!filter.trim();
        return (
          <div key={g.dir} className="work-fgroup">
            <button className="work-fgroup-head" aria-expanded={open} onClick={() => toggle(g.dir)}>
              <Icon name="chevron-right" size={11} />
              <span className="mono">{g.dir ? `${g.dir}/` : "top level"}</span>
              <span className="work-fgroup-n">{g.files.length}</span>
              <Lines additions={g.additions} deletions={g.deletions} />
            </button>
            {open && g.files.map((f) => <FileLine key={f.path} to={`/missions/${d.slug}/files/${c.repo}/${c.name}?${new URLSearchParams({ path: f.path })}`} f={f} dir={g.dir} committed={committed > 0} />)}
          </div>
        );
      })}
      {!groups.length && <p className="dim">Nothing matches.</p>}
      {!!more && !filter.trim() && <p className="dim">and {more.toLocaleString()} more</p>}
    </div>
  );
}

function FileLine({ to, f, dir, committed }: { to: string; f: ChangedFile; dir: string; committed: boolean }) {
  const rest = dir ? f.path.slice(dir.length + 1) : f.path;
  const i = rest.lastIndexOf("/");
  const body = (
    <>
      <span className={`work-file-st ${f.status}`}>{letters[f.status] ?? "M"}</span>
      <span className="work-file-path">
        <bdi>
          <span className="dim">{rest.slice(0, i + 1)}</span>
          <span className="work-file-name">{rest.slice(i + 1)}</span>
        </bdi>
      </span>
      {f.uncommitted && committed && <span className="work-file-loose" title="Not committed" />}
      <Lines additions={f.additions} deletions={f.deletions} />
    </>
  );
  const cls = `work-file ${f.status}${f.skim ? " skim" : ""}`;
  const title = `${f.oldPath ? `${f.oldPath} → ` : ""}${f.path}${f.uncommitted ? " · not committed" : ""}${f.skim ? ` · one to skim: ${f.skim}` : ""}`;
  // A file that's gone has nothing to open.
  return f.status === "deleted" ? (
    <div className={cls} title={title}>
      {body}
    </div>
  ) : (
    <Link to={to} className={cls} title={title}>
      {body}
    </Link>
  );
}

// The open output: what it is and where it came from, and its text to
// read. Its own page (⏎) is where it's commented on and edited.
function OutputPane({ d, a, home, label, select, open }: { d: MissionDetail; a: Artifact; home: Home; label: (repo: string) => string; select: (key: string) => void; open: (a: Artifact) => void }) {
  const reads = a.readable && !isArtifact(a);
  const { value: doc, error } = useLoaded(reads ? () => getArtifact(d.slug, a.ref) : null, [d.slug, a.ref, reads], { keep: true });
  const { title, desc } = artifactTitle(a);
  const handoff = a.kind === "handoff";
  // A handoff's text begins with its title and who left it where, which
  // the pane's head says.
  const text = doc?.text.replace(/^# .*\n+/, "").replace(/^Handoff from crew .*\n+/, "") ?? "";
  return (
    <>
      <header className="work-head">
        <div className="work-head-top">
          <span className="work-kind">
            {handoff ? "Handoff" : kindTitle(a.kind).replace(/s$/, "")}
            {home.step && ` · step ${home.step.n}, ${home.step.title}`} · <Time iso={a.added} />
          </span>
          <div className="work-head-actions">
            <Button size="small" onClick={() => open(a)} title={reads ? "Open its page, to comment on it or edit it (⏎)" : "Open it (⏎)"}>
              <Icon name={reads || isArtifact(a) ? "arrow-up-right" : "external"} /> {reads ? "Open its page" : "Open it"} <Keys k="⏎" />
            </Button>
          </div>
        </div>
        <h2 className="work-title">{handoff ? (desc ?? title) : title}</h2>
        <div className="work-head-meta">
          {(home.crew || a.by) && (
            <span>
              <CrewBadge id={home.crew?.id ?? a.by!} live={home.crew ? !home.crew.ended : false} /> {handoff ? "left it" : "made it"}
            </span>
          )}
          {home.capsule && (
            <Button variant="link" onClick={() => select(home.capsule!.key)} title={home.capsule.id}>
              <Icon name="box" size={12} /> {label(home.capsule.c.repo)}
              {home.capsule.c.name !== d.slug && <span className="mono"> {home.capsule.c.name}</span>}
            </Button>
          )}
          <ArtifactState state={a.state} />
          {a.threads && <ArtifactThreads t={a.threads} />}
          {!handoff && desc && <span>{desc}</span>}
        </div>
      </header>
      <div className="work-body one">
        <div className="work-col">
          {error && !doc && <ErrorCard title="Couldn't read it" error={error} />}
          {reads && !doc && !error && <Skeleton rows={6} />}
          {doc &&
            (doc.media ? (
              <Empty icon="file" title="It's an image or a video">
                Its page shows it as it is.
              </Empty>
            ) : doc.markdown ? (
              <div className="prose work-prose">
                <Markdown text={text} />
              </div>
            ) : (
              <pre className="artifact-code work-prose">{doc.text}</pre>
            ))}
          {!reads && (
            <Empty icon={isArtifact(a) ? "artifact" : "external"} title={isArtifact(a) ? "An artifact: it runs on its own page" : a.url ? "It's kept elsewhere" : "There's nothing of it to read here"}>
              {desc}
            </Empty>
          )}
        </div>
      </div>
    </>
  );
}
