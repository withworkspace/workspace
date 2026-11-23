import { type ReactNode, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { act, type MissionDetail, type Stage } from "../api";
import { passageChip, waiting } from "../ask/ask";
import { type Draft, MarkdownEditor } from "../editor/LazyEditor";
import { TrackedDoc } from "../editor/LazyTracked";
import { useKeys } from "../keys";
import { talkAt } from "../chrome/panes";
import { useTalk, useTalks } from "../conversation/talks";
import { type Reload, useHere } from "../load";
import { afterSettling } from "../talk/proposals";
import { Markdown } from "../markdown/LazyMarkdown";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { useMedia } from "../ui/kit";
import { toast } from "../ui/toast";
import { humanError } from "../ui/text";
import { Callout } from "../ui/Callout";
import { Keys } from "../ui/Keys";
import { Time } from "../ui/time";
import { BriefOutline, type DocId } from "./BriefOutline";
import { BriefPlanner, PassageTool, pickedIn, PlannerField, quoted } from "./BriefPlanner";
import { type Doc, type Edit, edits, planAsk, planSteps, reading, type Settled, shown } from "./briefpage";
import { Chip } from "../ui/Chip";
import { Button } from "../ui/Button";
import { TurnAvatar } from "../ui/Turn";
import { usePref } from "../prefs";

// A mission's Brief tab: its brief and its plan as documents, in the
// mission's own page. The brief is edited as it reads. The plan is read:
// ws holds it to a strict form, so it's rewritten by its planner, from the
// brief (Rewrite from the brief), and edited by hand only when asked for. Down the left, what they're made
// of (BriefOutline.tsx): the changes still to read, their sections, the
// plan's steps. Down the right, where the details are beside Summary, its
// planner's conversation (BriefPlanner.tsx): the one it was briefed in,
// carried on through it. One bar over them holds where they stand, the
// editor's toolbar (for whichever's being edited) and what there is to
// confirm.
//
// What the planner proposes for them isn't a card: it's the documents,
// its changes since the draft before marked in them, each to accept or
// decline. Confirming (briefing) or accepting (under way) settles the
// proposal as you left it; with none, your edits are saved as they are.
// Until anything's drafted, the conversation is in the documents' place.

export const briefKeys: [string, string][] = [
  ["j / k", "On Brief: the next change to read / the one before"],
  ["a / d", "On Brief: accept the change / decline it; with a passage picked out, a asks the planner about it"],
  ["/", "On Brief: ask the planner"],
  ["p", "On Brief: fold the planner's conversation away, or bring it back"],
  ["mod+⏎", "On Brief: confirm, accept or save (in the editor too)"],
];

// briefHref is a mission's brief: its page's Brief tab.
export const briefHref = (slug: string) => `/missions/${slug}?tab=brief`;

// DocState is where a document stands in the tab: what it comes to as
// you've left it, whether that's other than what it shows (you edited it,
// or declined a change), the changes still to read with the text they're
// marked in, whether it's the editor (not its changes) that's showing, and
// how its changes are settled.
interface DocState {
  text: () => string;
  changed: boolean;
  pending: Edit[];
  base: string;
  editing: boolean;
  settle: (id: string, a: "accept" | "decline") => void;
  all: (a: "accept" | "decline") => void;
}

interface Focus {
  doc: DocId;
  id: string;
}

// Wide enough for the brief and the plan side by side, each at a column
// worth reading: what the documents' pane has to be.
const sideBySide = 1120;

export function BriefTab({ d, root, updated, reload, onEscape }: { d: MissionDetail; root: string; updated?: string; reload: Reload; onEscape: () => void }) {
  const slug = d.slug;
  const navigate = useNavigate();
  const here = useHere();

  // Its planner's conversation, read again whenever the mission changes,
  // and followed while it's answered. It's the window's
  // (conversation/talks.ts): the same one its own page shows.
  const planner = d.planner;
  const to = planner ? talkAt(planner, slug) : undefined;
  const t = useTalk(to)?.detail;
  useEffect(() => {
    if (to) void useTalks.getState().load(to); // unread, the tab shows the mission without it
  }, [to, updated, d]);
  useEffect(() => (to ? useTalks.getState().watch(to) : undefined), [to]);
  const talks = useTalks.getState();
  const following = waiting(t);

  const s = useMemo(() => shown(d, t), [d, t]);
  // What each document comes to as you've left it, from its panel.
  const [brief, setBrief] = useState<DocState>();
  const [plan, setPlan] = useState<DocState>();
  // again starts both over from what's proposed (discarding your edits).
  const [again, setAgain] = useState(0);
  const [busy, setBusy] = useState(false);
  // Why ws didn't save your plan: every problem, shown under it.
  const [planProblem, setPlanProblem] = useState<string>();
  // The change in hand, of those still to read.
  const [focus, setFocus] = useState<Focus>();
  // The document last gone to, which the bar's toolbar is for.
  const [active, setActive] = useState<DocId>("brief");
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  // Writing it yourself, with nothing drafted.
  const [own, setOwn] = useState(false);
  const [beside, setBeside] = usePref("briefPlanner");
  const [held, setHeld] = useState<string>();
  const [asking, setAsking] = useState(false);
  const field = useRef<HTMLInputElement>(null);
  const docs = useRef<HTMLDivElement>(null);
  useEffect(() => {
    setOwn(false);
    setFocus(undefined);
    setHeld(undefined);
    setPlanProblem(undefined);
  }, [slug]);

  const p = s.proposal;
  const briefing = s.phase === "briefing";
  const ended = !!d.ending || d.state === "archived";
  const stale = !!(s.brief.stale || s.plan.stale);
  const changed = !!(brief?.changed || plan?.changed);
  const planned = !!s.plan.text.trim();
  // Something's there to read or write: a draft, a brief, or yours begun.
  const drafted = !briefing || !!p || !d.briefUnwritten || own;
  const pending: Focus[] = useMemo(
    () => [...(brief?.pending ?? []).map((e) => ({ doc: "brief" as const, id: e.id })), ...(plan?.pending ?? []).map((e) => ({ doc: "plan" as const, id: e.id }))],
    [brief?.pending, plan?.pending],
  );
  const toRead = pending.length;

  const done = (message: string) => toast(message, { tone: "done" });
  const failed = (e: Error) => toast(humanError(e.message), { tone: "danger" });
  const working = (st: DocState | undefined, doc: Doc) => (st ? st.text() : doc.text);

  // settle accepts the proposal (as you left its brief and plan) or
  // declines it. Briefing, accepting it starts the mission, and the drafts
  // it replaced are declined; the page goes to the mission's summary.
  const settle = async (accept: boolean) => {
    if (!p || !planner || busy) return;
    setBusy(true);
    try {
      const said: { brief?: string; plan?: string } = {};
      if (accept) {
        const b = working(brief, s.brief);
        const pl = working(plan, s.plan);
        if (p.kind === "brief" && b !== s.brief.text) said.brief = b;
        if (p.plan && pl !== s.plan.text) said.plan = pl;
      }
      const x = await talks.settle(talkAt(planner, slug), p.n, accept ? "accept" : "decline", said);
      // Confirming may have renamed it: its conversation's where it went.
      const moved = x.path?.match(/^\/missions\/([^/]+)\//)?.[1];
      if (accept && briefing) {
        // The drafts it replaced are nothing to confirm now.
        for (const o of s.others) await talks.settle(talkAt(planner, moved ?? slug), o.n, "decline").catch(() => undefined);
      }
      done(x.message ?? (accept ? "Done." : "Declined."));
      // On to the mission, or to where it's moved: if you're still here.
      if (!here()) return;
      if (accept && briefing) navigate(`/missions/${moved ?? slug}`);
      else if (moved && moved !== slug) navigate(briefHref(moved), { replace: true });
      else await reload();
    } catch (e) {
      failed(e as Error);
    } finally {
      setBusy(false);
    }
  };

  // save writes your edits of the brief and the plan as they are; then,
  // briefing with both written, confirm starts it.
  const save = async (thenConfirm: boolean) => {
    if (busy) return;
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
          await reload();
          return;
        }
      }
      if (thenConfirm) {
        const r = await act(slug, "confirm");
        done(r.message);
        if (here()) navigate(`/missions/${slug}`);
        return;
      }
      done(changed ? "Saved." : "Nothing to save.");
      setAgain((n) => n + 1);
      setOwn(false);
      await reload();
    } catch (e) {
      failed(e as Error);
    } finally {
      setBusy(false);
    }
  };

  // What the bar offers: the proposal to confirm or accept, or your edits
  // to save, or (briefing, its brief and plan written) to confirm them.
  // primary is mod+⏎, here and in the editors.
  const confirmable = briefing && !d.briefUnwritten && planned;
  const primary = ended
    ? undefined
    : p
      ? {
          label: briefing ? (p.kind === "plan" && d.briefUnwritten ? "Accept the plan" : "Confirm and start") : p.kind === "plan" ? "Accept the plan" : "Accept the edit",
          run: () => settle(true),
          disabled: stale,
        }
      : changed
        ? { label: confirmable ? "Save, and confirm and start" : "Save", run: () => save(confirmable), disabled: false }
        : confirmable
          ? { label: "Confirm and start", run: () => save(true), disabled: false }
          : undefined;

  // --- the planner -------------------------------------------------------

  // Asking it: said in its conversation, about a passage in hand if there
  // is one. A mission briefed without a conversation asks ws for a
  // planner, which reads the mission and revises its plan (replan).
  const canAsk = !ended && (!!planner || !briefing);
  const ask = (text: string) => {
    if (asking) return;
    const body = held ? quoted(held, text) : text;
    setAsking(true);
    const asked = to ? talks.say(to, body).then(() => undefined) : act(slug, "replan", { text: body }).then((r) => done(r.message));
    asked
      .then(() => {
        setHeld(undefined);
        return reload();
      }, failed)
      .finally(() => setAsking(false));
  };
  const briefIt = () => {
    if (busy) return;
    setBusy(true);
    act(slug, "brief")
      .then(() => reload(), failed)
      .finally(() => setBusy(false));
  };
  // rewritePlan has the planner write the plan again, to fit the brief as
  // you've left it. A brief that's written is saved first, with your
  // edits; a draft's isn't written until it's confirmed, so what you
  // changed in it goes with the ask. Under way it's a replan, which ws
  // applies, or brings back for you to accept.
  const canRewrite = !ended && !stale && (briefing ? !!planner : planned);
  const rewritePlan = async () => {
    if (!canRewrite || asking || busy) return;
    const now = working(brief, s.brief);
    if (!now.trim()) return toast("There's no brief yet for a plan to be written from.");
    setAsking(true);
    try {
      const draft = !!p && p.kind === "brief";
      if (brief?.changed && !draft) await act(slug, "edit-brief", { text: now });
      const body = planAsk({ planned, unseen: brief?.changed && draft ? now : undefined });
      if (briefing) await talks.say(to ?? "", body);
      else await act(slug, "replan", { text: body });
      setBeside(true);
      done(planned ? "Asked the planner to rewrite the plan. What it changes shows here, to read." : "Asked the planner for the plan. It shows here once it's proposed.");
      setOwn(false);
      await reload();
    } catch (e) {
      failed(e as Error);
    } finally {
      setAsking(false);
    }
  };
  const suggestions = held
    ? ["Why does it say this?", "Make this clearer", "Is this needed?"]
    : !planner
      ? []
      : briefing
        ? ["What's still open?", "Can Z be checked as it's written?", "What would you leave out?"]
        : ["Is the plan still right?", "What's changed since it started?"];
  const fold = setBeside;
  const toField = () => {
    if (!canAsk) return;
    // (Folded away, the field's at the documents' foot: it's there already.)
    setTimeout(() => field.current?.focus(), 0);
  };
  // askAbout takes a passage to the planner's field: the one given, or
  // what's picked out of the documents. It says whether there was one.
  const askAbout = (passage = pickedIn(docs.current)) => {
    if (!passage || !canAsk) return false;
    setHeld(passage);
    toField();
    return true;
  };
  // A card in the planner's conversation, settled there: confirming a
  // brief that starts the mission goes to its summary, as the bar's does.
  const settleOther = (n: number, action: "accept" | "decline", said?: object) => {
    const card = t?.proposals?.find((x) => x.n === n);
    return talks.settle(to ?? "", n, action, said).then((x) => {
      if (x.message) done(x.message);
      const to = afterSettling(card, action, slug, x.path);
      if (to && here()) return navigate(to, { replace: to === x.path });
      return reload();
    }, failed);
  };

  // --- the changes still to read ------------------------------------------

  const stateOf = (doc: DocId) => (doc === "brief" ? brief : plan);
  const at = focus ? pending.findIndex((x) => x.doc === focus.doc && x.id === focus.id) : -1;
  const move = (by: number) => {
    if (!pending.length) return;
    setFocus(pending[at < 0 ? (by > 0 ? 0 : pending.length - 1) : (at + by + pending.length) % pending.length]);
  };
  const settleInHand = (a: "accept" | "decline") => {
    if (!focus || at < 0) return;
    stateOf(focus.doc)?.settle(focus.id, a);
    setFocus(pending[at + 1] ?? pending[at - 1]);
  };
  const acceptAll = () => {
    brief?.all("accept");
    plan?.all("accept");
    setFocus(undefined);
  };

  useKeys({
    "/": toField,
    a: () => {
      if (!askAbout()) settleInHand("accept");
    },
    d: () => settleInHand("decline"),
    j: () => move(1),
    k: () => move(-1),
    p: () => fold(!beside),
    "mod+Enter": () => primary && !primary.disabled && primary.run(),
    Escape: () => {
      // In an editor or the planner's field, esc leaves it; your edits
      // stay, and aren't lost by going back.
      const el = document.activeElement as HTMLElement | null;
      if (el?.closest(".brief-doc, .brief-ask")) return el.blur();
      if (focus) return setFocus(undefined);
      if (held) return setHeld(undefined);
      if (changed) return toast("Your edits aren't saved: save them, or discard them, first.");
      onEscape();
    },
  });

  // --- how it's laid out ----------------------------------------------------

  // A wide window has the three panes, each scrolling by itself; a narrow
  // one is the documents, then the conversation, down the page.
  const wide = useMedia("(min-width: 1021px)");
  // The brief and the plan side by side, when the pane between the outline
  // and the planner has room for both.
  const [main, setMain] = useState<HTMLDivElement | null>(null);
  const [roomy, setRoomy] = useState(false);
  useEffect(() => {
    if (!main) return;
    const seen = new ResizeObserver(() => setRoomy(main.clientWidth >= sideBySide));
    seen.observe(main);
    return () => seen.disconnect();
  }, [main]);
  const side = wide && roomy && (planned || briefing);
  // The planner's pane: its conversation, or (briefed without one) where
  // one's asked for.
  const hasPlanner = !!planner || !ended;
  const open = beside && hasPlanner;
  // The toolbar starts where the documents' text does, however the panes
  // are laid out: measured, as the text's column is centred in its own.
  const [tools, setTools] = useState<HTMLDivElement | null>(null);
  const [inset, setInset] = useState(0);
  useLayoutEffect(() => {
    if (!main || !tools) return;
    // (A narrow window's toolbar is a row of its own, from the bar's edge.)
    if (!wide) return setInset(0);
    const look = () => {
      const head = main.querySelector(".brief-doc .brief-doc-head");
      if (head) setInset(Math.max(0, Math.round(head.getBoundingClientRect().left - tools.getBoundingClientRect().left)));
    };
    look();
    const seen = new ResizeObserver(look);
    seen.observe(main);
    seen.observe(tools);
    return () => seen.disconnect();
  }, [main, tools, side, open, wide]);

  // Which document the bar's toolbar is for: the one last gone to, if it's
  // being edited; else the other.
  const editing = (["brief", "plan"] as const).filter((x) => stateOf(x)?.editing);
  const toolsFor = editing.includes(active) ? active : editing[0];

  // The plan's steps, as it reads now: the outline's list follows it as
  // it's typed, and a step ws has keeps where the mission stands on it.
  const stages = useMemo(() => planSteps(plan ? plan.text() : s.plan.text, d.stages), [plan, s.plan.text, d.stages]);
  const now = d.stages.find((x) => x.state === "current");

  const what = (): { icon: IconName; text: ReactNode; title?: string; tone?: "attention" } => {
    if (ended) return { icon: "check-circle", text: "Ended: as it was written" };
    if (p) {
      const draft = Math.max(s.draft, 1);
      const name = briefing ? (p.kind === "plan" ? "The plan" : `Draft ${draft}`) : p.kind === "plan" ? "Plan revision" : "Brief edit";
      const title = briefing ? (p.kind === "plan" ? "The plan, proposed" : `Draft ${draft} of the brief${p.plan ? ", with its plan" : ""}`) : p.kind === "plan" ? "A revision of the plan" : "An edit to the brief";
      return {
        icon: p.kind === "plan" ? "list-ordered" : "pencil",
        tone: "attention",
        title,
        text: (
          <>
            {name}{" "}
            <span className="dim">
              · <Time iso={p.at} as="relative" />
            </span>
          </>
        ),
      };
    }
    if (changed) return { icon: "pencil", tone: "attention", text: "Your edits, not saved" };
    if (confirmable) return { icon: "check-circle", text: "Written", title: "Its brief and plan are written: confirming them starts it" };
    if (now)
      return {
        icon: "circle-dot",
        text: (
          <>
            As written{" "}
            <span className="dim">
              · at step {now.n} of {d.stages.length}
            </span>
          </>
        ),
      };
    return { icon: "file", text: briefing ? "To brief" : "As written" };
  };
  const w = what();

  const heldChip = held ? { ...passageChip(held), placeholder: "Ask about this passage…", drop: "Ask about the whole brief" } : undefined;
  const plannerField = canAsk ? (
    <PlannerField held={heldChip} dropHeld={() => setHeld(undefined)} suggestions={suggestions} ask={ask} input={field} busy={asking} placeholder={planner ? "Ask the planner…" : "What should change in the plan, and why"} />
  ) : null;
  const toDocs = () => (docs.current?.scrollTop ? docs.current.scrollTo({ top: 0, behavior: "smooth" }) : docs.current?.scrollIntoView({ block: "start", behavior: "smooth" }));
  const plannerPane = (centre?: boolean) =>
    planner && t ? (
      <BriefPlanner
        t={t}
        slug={slug}
        root={root}
        shown={p?.n}
        centre={centre}
        field={plannerField}
        // With nothing drafted, the brief can still be yours to write: its
        // planner stays beside it, to write the plan from it.
        action={
          centre && !ended ? (
            <Button variant="ghost" size="small" type="button" onClick={() => setOwn(true)} title="Write the brief yourself: its planner stays beside it, and can write the plan from it">
              <Icon name="pencil" size={12} /> Write it yourself
            </Button>
          ) : undefined
        }
        onSettle={settleOther}
        toDocs={toDocs}
        fold={centre || !wide ? undefined : () => fold(false)}
      />
    ) : planner ? (
      // Its conversation's being read: its place is kept, so the documents don't move when it's there.
      <aside className={`brief-planner${centre ? " centre" : ""}`} aria-label="The planner's conversation" aria-busy />
    ) : ended ? null : (
      <NoPlanner briefing={briefing} busy={busy} briefIt={briefIt} field={plannerField} fold={wide ? () => fold(false) : undefined} />
    );

  // Nothing drafted: the conversation is the page, or where it's begun.
  if (!drafted) {
    return (
      <div className="brief-tab talking">
        <div className="brief-panes">
          <aside className="brief-outline" aria-label="What the brief and plan are made of">
            <section>
              <h3>The brief</h3>
              <p className="map-empty">Not drafted yet. Its planner writes it with you: the goal, the context, what's in and out, and how you'll know it's done.</p>
            </section>
            <section>
              <h3>The plan</h3>
              <p className="map-empty">Comes with the brief: what Z is, where it may stop early, and its steps.</p>
            </section>
          </aside>
          <div className="brief-talk">
            {planner ? (
              plannerPane(true)
            ) : (
              <div className="brief-start">
                <span className="brief-start-icon">
                  <Icon name="pencil" size={18} />
                </span>
                <h2>Waiting to be briefed</h2>
                {d.request && <blockquote className="brief-request">{d.request}</blockquote>}
                <p className="dim">Its planner reads round what you asked for, asks what it needs to, and writes the brief and a plan with you. They're here to read, change and confirm.</p>
                <div className="brief-start-act">
                  <Button variant="primary" type="button" onClick={briefIt} disabled={busy} aria-busy={busy}>
                    {busy ? <Spinner /> : <Icon name="pencil" />} Brief it
                  </Button>
                  <Button variant="ghost" type="button" onClick={() => setOwn(true)}>
                    Write it yourself
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  const key = `${again}:${p?.n ?? "as-is"}`;
  // The plan is read, not typed in, until it's asked for: what's offered
  // of it is its planner writing it again from the brief.
  const guard: Guard = { rewrite: canRewrite ? rewritePlan : undefined, busy: asking || busy, planned };
  const panel = (id: DocId, title: string, doc: Doc, onState: (st: DocState) => void, empty?: string) => (
    <DocPanel
      key={`${id}:${key}`}
      id={id}
      title={title}
      doc={stale ? { text: doc.text } : doc}
      empty={empty}
      guard={id === "plan" ? guard : undefined}
      readOnly={ended}
      focus={focus?.doc === id ? focus.id : undefined}
      onFocus={(cid) => setFocus({ doc: id, id: cid })}
      onState={onState}
      onSubmit={primary && !primary.disabled ? primary.run : undefined}
      toolbarIn={slot}
      toolbarOn={toolsFor === id}
      onActive={() => setActive(id)}
    />
  );
  const nav = toRead > 0 && (
    <span className="brief-nav">
      <Chip tone="attention" className="brief-nav-chip">{at >= 0 ? `${at + 1} of ${toRead}` : `${toRead} ${toRead === 1 ? "change" : "changes"}`}</Chip>
      <Button variant="ghost" square type="button" onClick={() => move(-1)} title="The change before (k)" aria-label="The change before">
        <Icon name="chevron-up" size={13} />
      </Button>
      <Button variant="ghost" square type="button" onClick={() => move(1)} title="The next change (j)" aria-label="The next change">
        <Icon name="chevron-down" size={13} />
      </Button>
    </span>
  );
  // What the planner has for you while it's folded away: its turn to be
  // answered, or something it proposed that isn't the documents'.
  const plannerWaits = !!t && (following || (t.proposals ?? []).some((x) => x.state === "open" && x.kind !== "brief" && x.kind !== "plan"));

  return (
    <div className={`brief-tab${side ? " side" : ""}${open ? " planner-on" : ""}`}>
      <div className="brief-bar" role="region" aria-label="What's to confirm">
        <div className={`brief-what${w.tone ? ` tone-${w.tone}` : ""}`} title={w.title}>
          <Icon name={w.icon} size={13} />
          <span className="brief-what-text">{w.text}</span>
        </div>
        <div className="brief-toolbar" ref={setTools} style={{ paddingLeft: inset }}>
          {toolsFor && editing.length === 1 && toRead > 0 && (
            <span className="brief-toolbar-for" title="The other's changes are still to read: settled, it's yours to edit too">
              {toolsFor === "plan" ? "The plan" : "The brief"}
            </span>
          )}
          <div className="brief-toolbar-slot" ref={setSlot} />
          {!toolsFor && <span className="brief-toolbar-none dim">{ended ? "It's ended: nothing to edit" : toRead ? "Settle a document's changes, and it's yours to edit" : ""}</span>}
        </div>
        <div className="brief-act">
          {busy && <Spinner />}
          {nav}
          {changed && (
            <Button variant="ghost" type="button" disabled={busy} onClick={() => setAgain((n) => n + 1)} title={p ? "Start over from what's proposed" : "Start over from what's written"}>
              Discard my edits
            </Button>
          )}
          {p && (
            <Button variant="ghost" type="button" disabled={busy} onClick={() => settle(false)} title={briefing ? "Decline this draft: nothing's written" : "Decline it: it stays as it's written"}>
              Decline
            </Button>
          )}
          {primary && (
            <Button
              variant="primary"
              type="button"
              disabled={busy || primary.disabled}
              onClick={primary.run}
              title={briefing && p ? "Nothing's written until you confirm it: your edits, and the changes you decline, go with it. The planner doesn't see them until then" : undefined}
            >
              {primary.label} <Keys k="mod+⏎" />
            </Button>
          )}
          {wide && !open && hasPlanner && (
            <Button variant="ghost" square type="button" className="brief-unfold" onClick={() => fold(true)} title="Show the planner's conversation (p)" aria-label="Show the planner's conversation">
              <Icon name="panel" size={14} />
              {plannerWaits && <span className="brief-unfold-dot" aria-label="It has something for you" />}
            </Button>
          )}
        </div>
      </div>

      <div className="brief-panes">
        <BriefOutline
          docs={docs}
          briefing={briefing}
          brief={brief}
          plan={plan}
          stages={stages}
          planned={planned || (!!plan && !!plan.text().trim())}
          fresh={briefing && !!p?.plan && s.plan.was === undefined}
          at={at}
          move={move}
          focus={(doc, id) => setFocus({ doc, id })}
          acceptAll={acceptAll}
          now={!briefing && !ended ? now : undefined}
          request={d.request}
        />

        <div className="brief-main" ref={setMain}>
          <div className="brief-docs" ref={docs}>
            {stale && (
              <Callout className="brief-docs-note" tone="attention" icon="alert" title="Proposed against an earlier version">
                It's changed since, so accepting this is refused. Ask the planner for it again, from what's written now.
              </Callout>
            )}
            {panel("brief", "The brief", s.brief, setBrief)}
            {(planned || briefing) && (
              <>
                {d.planProblem && (
                  <Callout className="brief-docs-note" tone="attention" icon="alert" title="The plan doesn't read, so the mission works from its brief">
                    {d.planProblem}
                  </Callout>
                )}
                {panel("plan", "The plan", s.plan, setPlan, briefing ? (planner ? "No plan yet: its planner writes one with the brief, or from a brief of yours." : "No plan yet: its planner writes one with the brief.") : undefined)}
              </>
            )}
            {planProblem && (
              <Callout className="brief-docs-note" tone="danger" icon="alert" title="ws didn't save the plan">
                <pre className="plan-problem">{planProblem}</pre>
              </Callout>
            )}
            <PlanMarks stages={d.stages} on={!briefing && !ended && !plan?.pending.length} />
            <PassageTool within={docs} ask={askAbout} />
          </div>
          {wide && !open && canAsk && <div className="brief-ask-foot">{plannerField}</div>}
        </div>

        {(open || !wide) && plannerPane()}
      </div>
    </div>
  );
}

// NoPlanner is where a mission that was briefed without a conversation
// gets a planner: briefing, by having it briefed; under way, by saying
// what should change in the plan, which ws puts to a planner of its own.
function NoPlanner({ briefing, busy, briefIt, field, fold }: { briefing: boolean; busy: boolean; briefIt: () => void; field: ReactNode; fold?: () => void }) {
  return (
    <aside className="brief-planner" aria-label="The planner">
      <div className="brief-planner-head">
        <TurnAvatar by="agent" />
        <span className="brief-planner-name">Planner</span>
        <span className="spacer" />
        {fold && (
          <Button variant="ghost" square type="button" onClick={fold} title="Fold the planner away (p)" aria-label="Fold the planner away">
            <Icon name="panel" size={14} />
          </Button>
        )}
      </div>
      <div className="brief-planner-body">
        {briefing ? (
          <>
            <p className="dim">It's been written without a planner. Have it briefed, and a planner reads round it, asks what it needs to, and proposes its brief and a plan here.</p>
            <div>
              <Button type="button" onClick={briefIt} disabled={busy} aria-busy={busy}>
                <Icon name="pencil" /> Brief it
              </Button>
            </div>
          </>
        ) : (
          <p className="dim">It was briefed without a conversation. Say what should change, and a planner reads the mission and revises its plan. The steps behind the mission stay as they are. A change to a checkpoint or Z comes back to you to accept; anything else applies.</p>
        )}
      </div>
      <div className="brief-planner-foot">{field}</div>
    </aside>
  );
}

// Guard is a document that's read until editing it by hand is asked for
// (the plan, which ws holds to a strict form): what's offered instead is
// having it written again (rewrite; none when there's nobody to ask),
// busy while that's being asked. planned: there's one to rewrite.
interface Guard {
  rewrite?: () => void;
  busy: boolean;
  planned: boolean;
}

// DocPanel is the brief or the plan: its changes since the version before
// marked in it to accept or decline, one by one or all at once; then (or
// with none) the document itself, in the editor, to change as it reads
// (a guarded one, as it reads, until it's opened to edit by hand). Its
// toolbar is the tab's one bar's, while it's the one being edited.
function DocPanel({
  id,
  title,
  doc,
  empty,
  guard,
  readOnly,
  focus,
  onFocus,
  onState,
  onSubmit,
  toolbarIn,
  toolbarOn,
  onActive,
}: {
  id: DocId;
  title: string;
  doc: Doc;
  empty?: string;
  guard?: Guard;
  readOnly?: boolean;
  focus?: string;
  onFocus: (id: string) => void;
  onState: (s: DocState) => void;
  onSubmit?: () => void;
  toolbarIn: HTMLElement | null;
  toolbarOn: boolean;
  onActive: () => void;
}) {
  const list = useMemo(() => (doc.was !== undefined && doc.was !== doc.text ? edits(doc.was, doc.text) : []), [doc.was, doc.text]);
  const [settled, setSettled] = useState<Settled>({});
  const [unplaced, setUnplaced] = useState(0);
  const [draft, setDraft] = useState<Draft>();
  // Opened to edit by hand (a guarded document's only: any other is).
  const [byHand, setByHand] = useState(!guard);
  const r = useMemo(() => reading(doc.was ?? doc.text, list, settled), [doc.was, doc.text, list, settled]);
  const showing = r.result ?? doc.text;
  const toSettle = r.pending.length > 0;
  // Nothing in it, and something to say of that: said, until it's opened
  // to write by hand.
  const blank = !showing.trim() && !!empty && !(guard && byHand);
  const settle = useCallback((cid: string, a: "accept" | "decline") => setSettled((x) => ({ ...x, [cid]: a })), []);
  const all = useCallback((a: "accept" | "decline") => setSettled((x) => ({ ...Object.fromEntries(list.map((e) => [e.id, a])), ...x })), [list]);
  useEffect(() => {
    onState({
      text: () => (draft?.dirty ? draft.text() : r.comes),
      changed: !!draft?.dirty || r.comes !== doc.text,
      pending: r.pending,
      base: r.base,
      editing: !toSettle && !blank && !readOnly && byHand,
      settle,
      all,
    });
  }, [r, draft, doc.text, onState, toSettle, blank, readOnly, byHand, settle, all]);
  const tracked = useMemo(() => r.pending.map((e) => ({ ...e, by: "agent" })), [r.pending]);
  return (
    <section className="brief-doc" data-doc={id}>
      <h2 className="brief-doc-head">
        {title}
        {toSettle && (
          <Chip tone="attention" className="brief-head-chip">
            {r.pending.length} {r.pending.length === 1 ? "change" : "changes"} to read
          </Chip>
        )}
        <span className="spacer" />
        {toSettle && (
          <>
            <Button variant="ghost" size="small" type="button" onClick={() => all("decline")} title="Leave each as the draft before had it">
              Decline the rest
            </Button>
            <Button size="small" type="button" onClick={() => all("accept")} title="Take each as it's proposed">
              Accept the rest
            </Button>
          </>
        )}
        {guard && !toSettle && !readOnly && !byHand && (
          <>
            <Button
              variant="ghost" size="small"
              type="button"
              onClick={() => setByHand(true)}
              title="Change the plan yourself. ws holds a plan to a strict form: one it can't read is refused when you save it, saying every problem"
            >
              <Icon name="pencil" size={12} /> Edit by hand
            </Button>
            {guard.rewrite && (
              <Button
                size="small"
                type="button"
                disabled={guard.busy}
                aria-busy={guard.busy}
                onClick={guard.rewrite}
                title={guard.planned ? "Have its planner write the plan again, to fit the brief as it reads now (your edits to the brief go with it)" : "Have its planner write the plan, from the brief as it reads now"}
              >
                {guard.busy ? <Spinner size={12} /> : <Icon name="sparkle" size={12} />} {guard.planned ? "Rewrite from the brief" : "Write it from the brief"}
              </Button>
            )}
          </>
        )}
      </h2>
      {unplaced > 0 && <p className="dim brief-note">{unplaced === 1 ? "A change couldn't be shown in place: it's in as proposed." : `${unplaced} changes couldn't be shown in place: they're in as proposed.`}</p>}
      {toSettle ? (
        <TrackedDoc
          text={r.base}
          changes={tracked}
          focus={focus}
          onFocus={onFocus}
          onSettle={settle}
          onUnplaced={(ids) => {
            setUnplaced((n) => n + ids.length);
            setSettled((x) => ({ ...x, ...Object.fromEntries(ids.map((cid) => [cid, "accept" as const])) }));
          }}
        />
      ) : blank ? (
        <div className="brief-waiting dim">{empty}</div>
      ) : readOnly || !byHand ? (
        <div className="prose">
          <Markdown text={showing} />
        </div>
      ) : (
        <div className="brief-editor">
          {guard && (
            <p className="dim brief-note">
              ws holds a plan to a strict form: each step starts with its kind, and what's under it is a line each. One it can't read is refused when you save it, saying every problem. <em>Discard my edits</em> puts it back.
            </p>
          )}
          <MarkdownEditor
            key={showing}
            initial={showing}
            label={title}
            placeholder={guard ? "## Z, then ## Stops early, then ## Steps: a numbered list, each step starting with its kind" : (empty ?? "The goal, the context, what's in and out, and how you'll know it's done")}
            autoFocus={!!guard}
            onChange={setDraft}
            onSubmit={onSubmit}
            toolbarIn={toolbarIn}
            toolbarOn={toolbarOn}
            onFocus={onActive}
          />
        </div>
      )}
    </section>
  );
}

// PlanMarks marks, in the plan as it reads, where the mission is on it:
// the steps behind it, and the one it's at with who's on it. (A revision
// changes only what's ahead.) They're the plan's own list items, marked by
// where they come in it, so the editor's left to draw them.
function PlanMarks({ stages, on }: { stages: Stage[]; on: boolean }) {
  if (!on || !stages.length) return null;
  const li = (n: number) => `.brief-doc[data-doc="plan"] .markdown > ol:first-of-type > li:nth-child(${n})`;
  const done = stages.filter((s) => s.state === "accepted").map((s) => li(s.n));
  const now = stages.find((s) => s.state === "current");
  const says = (s: string) => `"${s.replace(/["\\]/g, "")}"`;
  const css = [
    done.length ? `${done.map((x) => `${x}::before`).join(",")}{content:"Done";background:var(--well);color:var(--dim);}` : "",
    done.length ? `${done.join(",")}{color:var(--dim);}` : "",
    now ? `${li(now.n)}::before{content:${says(`Now${now.crew?.length ? ` · ${now.crew.join(", ")}` : ""}`)};}` : "",
    now ? `${li(now.n)}{box-shadow:-3px 0 0 var(--progress);}` : "",
  ].join("");
  return <style>{css}</style>;
}
