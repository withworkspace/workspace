import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { act, getMission, getTalk, type MissionDetail, sayInTalk, settleTalkProposal, type TalkDetail, type TalkProposal, useEvents, type Workspace } from "../api";
import { AskBar, type AskBarHandle } from "../ask/AskBar";
import { type AskAbout, passageChip, readsAs } from "../ask/ask";
import { type Draft, MarkdownEditor } from "../editor/LazyEditor";
import { TrackedDoc } from "../editor/LazyTracked";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Callout, ErrorCard, humanError, Keys, Skeleton, toast, useTitle } from "../ui/kit";
import { Time } from "../ui/time";
import { type Doc, edits, reading, type Settled, shown } from "./briefpage";

export const briefKeys: [string, string][] = [
  ["/", "Ask the planner"],
  ["a", "Ask about what's selected"],
  ["mod+⏎", "Confirm, accept or save (in the editor too)"],
  ["esc", "Drop the answer, or back to the mission"],
];

// briefHref is a mission's brief's own page.
export const briefHref = (slug: string) => `/missions/${slug}/brief`;

// BriefView is a mission's brief and plan on a page of their own, as
// documents to read and edit in place, with the ask bar at its foot talking
// in its planner's conversation (the one it was briefed in, carried on
// through it). What the planner proposes for them isn't a card: it's the
// documents, its changes since the draft before marked in them, each to
// accept or decline. Confirming (briefing) or accepting (under way) settles
// the proposal as you left it; with none, your edits are saved as they are.
export function BriefView({ workspace }: { workspace: Workspace }) {
  const { slug = "" } = useParams();
  const navigate = useNavigate();
  const updated = workspace.missions.find((m) => m.slug === slug)?.updated;
  const { value: d, error, reload } = useLoaded<MissionDetail>(() => getMission(slug, false), [slug], { keep: true });
  useEvents(`mission:${slug}`, { changed: reload });
  useTitle(d ? `Brief · ${d.title}` : "Brief");

  // Its planner's conversation, read again whenever the mission changes
  // (the ask bar follows it while it's answered).
  const planner = d?.planner;
  const [t, setT] = useState<TalkDetail>();
  useEffect(() => {
    if (!planner) return setT(undefined);
    let stale = false;
    getTalk(slug, planner).then(
      (x) => !stale && setT(x),
      () => undefined, // the page shows the mission without it
    );
    return () => {
      stale = true;
    };
  }, [slug, planner, updated, d]);

  const s = useMemo(() => (d ? shown(d, t) : undefined), [d, t]);
  // What else its planner's conversation proposed, open: the conversation's
  // to confirm, not this page's.
  const aside = (t?.proposals ?? []).filter((x) => x.state === "open" && x.kind !== "brief" && x.kind !== "plan").length;
  // What each document comes to as you've left it, from its panel.
  const [brief, setBrief] = useState<DocState>();
  const [plan, setPlan] = useState<DocState>();
  // again starts both over from what's proposed (discarding your edits).
  const [again, setAgain] = useState(0);
  const [busy, setBusy] = useState(false);
  // Why ws didn't save your plan: every problem, shown under it.
  const [planProblem, setPlanProblem] = useState<string>();
  const asker = useRef<AskBarHandle<string>>(null);

  const p = s?.proposal;
  const briefing = s?.phase === "briefing";
  const ended = !!d?.ending || d?.state === "archived";
  const stale = !!(s?.brief.stale || s?.plan.stale);
  const changed = !!(brief?.changed || plan?.changed);
  const toRead = (brief?.pending ?? 0) + (plan?.pending ?? 0);
  const planned = !!s?.plan.text.trim();

  const done = (message: string) => toast(message, { tone: "done" });
  const failed = (e: Error) => toast(humanError(e.message), { tone: "danger" });
  const working = (st: DocState | undefined, doc: Doc) => (st ? st.text() : doc.text);

  // settle accepts the proposal (as you left its brief and plan) or
  // declines it. Briefing, accepting it starts the mission, and the drafts
  // it replaced are declined; the page goes to the mission.
  const settle = async (accept: boolean) => {
    if (!s || !p || !planner || busy) return;
    setBusy(true);
    try {
      const said: { brief?: string; plan?: string } = {};
      if (accept) {
        const b = working(brief, s.brief);
        const pl = working(plan, s.plan);
        if (p.kind === "brief" && b !== s.brief.text) said.brief = b;
        if (p.plan && pl !== s.plan.text) said.plan = pl;
      }
      const x = await settleTalkProposal(slug, planner, p.n, accept ? "accept" : "decline", said);
      // Confirming may have renamed it: its conversation's where it went.
      const moved = x.path?.match(/^\/missions\/([^/]+)\//)?.[1];
      if (accept && briefing) {
        // The drafts it replaced are nothing to confirm now.
        for (const o of s.others) await settleTalkProposal(moved ?? slug, planner, o.n, "decline").catch(() => undefined);
      }
      done(x.message ?? (accept ? "Done." : "Declined."));
      setT(x);
      if (accept && briefing) navigate(`/missions/${moved ?? slug}`);
      else if (moved && moved !== slug) navigate(briefHref(moved), { replace: true });
      else reload();
    } catch (e) {
      failed(e as Error);
    } finally {
      setBusy(false);
    }
  };

  // save writes your edits of the brief and the plan as they are; then,
  // briefing with both written, confirm starts it.
  const save = async (thenConfirm: boolean) => {
    if (!s || busy) return;
    setBusy(true);
    try {
      if (brief?.changed) await act(slug, "edit-brief", { text: brief.text() });
      if (plan?.changed) {
        try {
          await act(slug, "edit-plan", { text: plan.text() });
          setPlanProblem(undefined);
        } catch (e) {
          // A plan ws can't read is refused, saying why: it's shown under
          // the plan, to fix there (the brief, if changed, is saved).
          setPlanProblem(humanError((e as Error).message));
          if (brief?.changed) done("Saved the brief. The plan wasn't: see why under it.");
          reload();
          return;
        }
      }
      if (thenConfirm) {
        const r = await act(slug, "confirm");
        done(r.message);
        navigate(`/missions/${slug}`);
        return;
      }
      done(changed ? "Saved." : "Nothing to save.");
      setAgain((n) => n + 1);
      reload();
    } catch (e) {
      failed(e as Error);
    } finally {
      setBusy(false);
    }
  };

  // What the page's bar offers: the proposal to confirm or accept, or your
  // edits to save, or (briefing, its brief and plan written) to confirm
  // them. primary is mod+⏎, here and in the editors.
  const confirmable = briefing && !d?.briefUnwritten && planned;
  const primary = ended
    ? undefined
    : p
      ? {
          label: briefing ? (p.kind === "plan" && d?.briefUnwritten ? "Accept the plan" : "Confirm and start") : p.kind === "plan" ? "Accept the plan" : "Accept the edit",
          run: () => settle(true),
          disabled: stale,
        }
      : changed
        ? { label: confirmable ? "Save, and confirm and start" : "Save", run: () => save(confirmable), disabled: false }
        : confirmable
          ? { label: "Confirm and start", run: () => save(true), disabled: false }
          : undefined;

  const about: AskAbout<string> = {
    title: "the brief",
    start: (body, quote) => sayInTalk(slug, planner ?? "", quote ? `${quoted(quote)}\n\n${body}` : body),
    chip: (q) => ({ ...passageChip(q), placeholder: "Ask about this passage…", drop: "Ask about the whole brief" }),
  };
  const suggest = (q?: string) =>
    q
      ? ["Why does it say this?", "Make this clearer", "Is this needed?"]
      : briefing
        ? ["What's still open?", "Can Z be checked as it's written?", "What would you leave out?"]
        : ["Is the plan still right?", "What's changed since it started?"];
  // a asks about what's picked out of the documents, or all of it.
  const askAbout = () => {
    const sel = window.getSelection();
    const inDocs = sel && !sel.isCollapsed && sel.anchorNode && (sel.anchorNode as Element).parentElement?.closest(".brief-doc");
    const picked = inDocs ? readsAs(sel.toString()) : "";
    asker.current?.ask(picked.length >= 3 ? picked : undefined);
  };
  useKeys({
    "/": () => asker.current?.focus(),
    a: askAbout,
    "mod+Enter": () => primary && !primary.disabled && primary.run(),
    Escape: () => {
      // In an editor, esc leaves it; your edits stay, and aren't lost by
      // going back.
      const at = document.activeElement as HTMLElement | null;
      if (at?.closest(".brief-doc")) return at.blur();
      if (asker.current?.close()) return;
      if (changed) return toast("Your edits aren't saved: save them, or discard them, first.");
      navigate(`/missions/${slug}`);
    },
  });

  if (error) return <ErrorCard error={error} retry={reload} />;
  if (!d || !s) return <Skeleton rows={10} />;

  const phase = ended ? "Ended" : briefing ? (p || confirmable ? "To confirm" : "To brief") : "Under way";
  const drafting = briefing && !p && d.briefUnwritten;
  const key = `${again}:${p?.n ?? "as-is"}`;
  return (
    <div className="page narrow brief-view">
      <header className="talk-head brief-head">
        <Link to={`/missions/${slug}`} className="ghost-link back" title="Back to the mission (esc)">
          <Icon name="arrow-left" size={15} />
          <span className="sr-only">Back</span>
        </Link>
        <div className="talk-title">
          {/* Briefing, it's named as it's proposed, when it's confirmed. */}
          <h1 title={briefing && p?.title && p.title !== d.title ? `Named this when it's confirmed: it's “${d.title}” until then` : undefined}>{briefing && p?.title ? p.title : d.title}</h1>
          <div className="talk-sub">
            <span className={`chip ${phase === "To confirm" ? "tone-attention" : "tone-quiet"}`}>{phase}</span>
            {planner ? (
              <Link to={`/missions/${slug}/conversations/${planner}`} className="talk-mission" title="The whole conversation with its planner">
                <Icon name="message" size={11} /> {planner}, its planner's conversation
              </Link>
            ) : (
              <span className="dim">Briefed without a conversation</span>
            )}
          </div>
        </div>
      </header>

      {briefing && d.request && (
        <Callout tone="quiet" icon="message" title="Requested">
          <Inline text={d.request} />
        </Callout>
      )}

      {(primary || p || changed) && (
        <div className="brief-bar" role="region" aria-label="What's to confirm">
          <span className="brief-bar-what">
            {p ? (
              <>
                <Icon name={p.kind === "plan" ? "list-ordered" : "pencil"} size={13} />{" "}
                {briefing ? (s.draft > 1 ? `Draft ${s.draft} of the brief` : p.kind === "plan" ? "The plan" : "The brief and its plan") : p.kind === "plan" ? "A revision of the plan" : "An edit to the brief"}, proposed <Time iso={p.at} as="relative" />
                {toRead > 0 && (
                  <span className="dim">
                    {" "}
                    · {toRead} {toRead === 1 ? "change" : "changes"} {briefing ? "since the draft before" : "to what's written"}
                  </span>
                )}
              </>
            ) : changed ? (
              <>
                <Icon name="pencil" size={13} /> Your edits, not saved
              </>
            ) : (
              <>
                <Icon name="check-circle" size={13} /> Its brief and plan are written: confirming them starts it
              </>
            )}
          </span>
          <span className="spacer" />
          {busy && <Icon name="spinner" className="spin" />}
          {changed && (
            <button className="ghost" disabled={busy} onClick={() => setAgain((n) => n + 1)} title={p ? "Start over from what's proposed" : "Start over from what's written"}>
              Discard my edits
            </button>
          )}
          {p && (
            <button className="ghost" disabled={busy} onClick={() => settle(false)}>
              Decline
            </button>
          )}
          {primary && (
            <button className="primary" disabled={busy || primary.disabled} onClick={primary.run}>
              {primary.label} <Keys k="mod+⏎" />
            </button>
          )}
        </div>
      )}
      {stale && (
        <Callout tone="attention" icon="alert" title="Proposed against an earlier version">
          It's changed since, so accepting this is refused. Ask the planner below for it again, from what's written now.
        </Callout>
      )}
      {briefing && p && <p className="dim brief-note">Nothing's written until you confirm it: your edits, and the changes you decline, go with it. The planner doesn't see them until then.</p>}
      {aside > 0 && planner && (
        <Callout tone="attention" icon="reply" title={`${planner} proposes ${aside === 1 ? "something else" : `${aside} other things`} to confirm`}>
          Crew, a decision or a mission aren't the brief's: confirm or decline {aside === 1 ? "it" : "them"} in{" "}
          <Link to={`/missions/${slug}/conversations/${planner}`} className="link">
            the conversation
          </Link>
          .
        </Callout>
      )}

      {drafting ? (
        <section className="brief-doc">
          <h2 className="brief-doc-head">The brief</h2>
          <div className="brief-waiting dim">
            {t?.answering ? (
              <>
                <Icon name="spinner" className="spin" /> Its planner's drafting the brief and its plan: they'll be here to read, change and confirm.
              </>
            ) : (
              "Nothing's drafted yet. Ask its planner below for a brief and a plan."
            )}
          </div>
        </section>
      ) : (
        <>
          <DocPanel key={`b:${key}`} title="The brief" doc={stale ? { text: s.brief.text } : s.brief} onState={setBrief} onSubmit={primary?.run} />
          {(planned || briefing) && (
            <DocPanel
              key={`p:${key}`}
              title="The plan"
              doc={stale ? { text: s.plan.text } : s.plan}
              onState={setPlan}
              onSubmit={primary?.run}
              empty={briefing ? "No plan yet: its planner writes one with the brief." : undefined}
            />
          )}
          {planProblem && (
            <Callout tone="danger" icon="alert" title="ws didn't save the plan">
              <pre className="plan-problem">{planProblem}</pre>
            </Callout>
          )}
        </>
      )}

      {planner ? (
        <AskBar
          ref={asker}
          about={about}
          suggestions={suggest}
          root={workspace.root}
          in={{
            slug,
            id: planner,
            detail: t,
            href: `/missions/${slug}/conversations/${planner}`,
            label: `In ${planner}: what it proposes for the brief and plan is in them, above.`,
            draws: (x: TalkProposal) => x.kind !== "brief" && x.kind !== "plan",
            onTalk: setT,
          }}
        />
      ) : (
        briefing && (
          <div className="brief-waiting">
            <button className="primary" onClick={() => act(slug, "brief").then((r) => (r.path ? navigate(r.path, { replace: true }) : reload()), failed)}>
              <Icon name="pencil" /> Brief it
            </button>
          </div>
        )
      )}
    </div>
  );
}

// quoted is a passage as a message quotes it.
const quoted = (s: string) =>
  s
    .split("\n")
    .map((l) => `> ${l}`)
    .join("\n");

// DocState is where a document stands on the page: what it comes to as
// you've left it, whether that's other than what it shows (you edited it,
// or declined a change), and how many changes are still to read.
interface DocState {
  text: () => string;
  changed: boolean;
  pending: number;
}

// DocPanel is the brief or the plan: its changes since the version before
// marked in it to accept or decline, one by one or all at once; then (or
// with none) the document itself, in the editor, to change as it reads.
function DocPanel({ title, doc, empty, onState, onSubmit }: { title: string; doc: Doc; empty?: string; onState: (s: DocState) => void; onSubmit?: () => void }) {
  const list = useMemo(() => (doc.was !== undefined && doc.was !== doc.text ? edits(doc.was, doc.text) : []), [doc.was, doc.text]);
  const [settled, setSettled] = useState<Settled>({});
  const [focus, setFocus] = useState<string>();
  const [unplaced, setUnplaced] = useState(0);
  const [draft, setDraft] = useState<Draft>();
  const r = useMemo(() => reading(doc.was ?? doc.text, list, settled), [doc.was, doc.text, list, settled]);
  const showing = r.result ?? doc.text;
  useEffect(() => {
    onState({ text: () => (draft?.dirty ? draft.text() : r.comes), changed: !!draft?.dirty || r.comes !== doc.text, pending: r.pending.length });
  }, [r, draft, doc.text, onState]);
  const all = (a: "accept" | "decline") => setSettled((x) => ({ ...Object.fromEntries(list.map((e) => [e.id, a])), ...x }));
  const reading_ = r.pending.length > 0;
  return (
    <section className="brief-doc">
      <h2 className="brief-doc-head">
        {title}
        {reading_ && (
          <span className="chip tone-attention">
            {r.pending.length} {r.pending.length === 1 ? "change" : "changes"} to read
          </span>
        )}
        <span className="spacer" />
        {reading_ && (
          <>
            <button className="small ghost" onClick={() => all("decline")} title="Leave each as the draft before had it">
              Decline the rest
            </button>
            <button className="small" onClick={() => all("accept")} title="Take each as it's proposed">
              Accept the rest
            </button>
          </>
        )}
      </h2>
      {unplaced > 0 && (
        <p className="dim brief-note">
          {unplaced === 1 ? "A change couldn't be shown in place: it's in as proposed." : `${unplaced} changes couldn't be shown in place: they're in as proposed.`}
        </p>
      )}
      {reading_ ? (
        <TrackedDoc
          text={r.base}
          changes={r.pending.map((e) => ({ ...e, by: "agent" }))}
          focus={focus}
          onFocus={setFocus}
          onSettle={(id, a) => setSettled((x) => ({ ...x, [id]: a }))}
          onUnplaced={(ids) => {
            setUnplaced((n) => n + ids.length);
            setSettled((x) => ({ ...x, ...Object.fromEntries(ids.map((id) => [id, "accept" as const])) }));
          }}
        />
      ) : !showing.trim() && empty ? (
        <div className="brief-waiting dim">{empty}</div>
      ) : (
        <div className="brief-editor">
          <MarkdownEditor key={showing} initial={showing} label={title} placeholder={empty} onChange={setDraft} onSubmit={onSubmit} />
        </div>
      )}
    </section>
  );
}
