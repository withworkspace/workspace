import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { act, type MissionDetail } from "../../api";
import { passageChip, waiting } from "../../ask/ask";
import { talkAt } from "../../chrome/panes";
import { useTalk, useTalks } from "../../conversation/talks";
import { useKeys } from "../../keys";
import { type Reload, useHere } from "../../load";
import { usePref } from "../../prefs";
import { afterSettling } from "../../talk/proposals";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Spinner } from "../../ui/Spinner";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { useMedia } from "../../ui/useMedia";
import { type Doc, planAsk, planSteps, shown } from "../briefpage";
import { BriefDocPanel } from "./BriefDocPanel";
import { BriefNoPlanner } from "./BriefNoPlanner";
import { BriefOutline } from "./BriefOutline";
import { BriefPassageTool } from "./BriefPassageTool";
import { BriefPlanMarks } from "./BriefPlanMarks";
import { BriefPlanner } from "./BriefPlanner";
import { BriefPlannerField } from "./BriefPlannerField";
import { BriefWhat } from "./BriefWhat";
import type { DocId, DocState, Focus, Guard } from "./docState";
import { pickedIn, writing } from "./find";
import { briefHref } from "./href";
import { MissionBrief } from "./MissionBrief";
import { MissionBriefActs } from "./MissionBriefActs";
import { MissionBriefAskFoot } from "./MissionBriefAskFoot";
import { MissionBriefBar } from "./MissionBriefBar";
import { MissionBriefDocs } from "./MissionBriefDocs";
import { MissionBriefDocsNote } from "./MissionBriefDocsNote";
import { MissionBriefMain } from "./MissionBriefMain";
import { MissionBriefNav } from "./MissionBriefNav";
import { MissionBriefOutline } from "./MissionBriefOutline";
import { MissionBriefOutlineEmpty } from "./MissionBriefOutlineEmpty";
import { MissionBriefOutlineSection } from "./MissionBriefOutlineSection";
import { MissionBriefPlanner } from "./MissionBriefPlanner";
import { MissionBriefPlanProblem } from "./MissionBriefPlanProblem";
import { MissionBriefStart } from "./MissionBriefStart";
import { MissionBriefTalk } from "./MissionBriefTalk";
import { MissionBriefToolbar } from "./MissionBriefToolbar";
import { MissionBriefUnfold } from "./MissionBriefUnfold";
import { useInset } from "./useInset";
import { useRoom } from "./useRoom";
import { acceptLabel, quoted, suggestions as suggestionsFor } from "./words";

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
//
// This is what the tab knows and does: the mission and its planner's
// conversation, where each document stands, the change in hand, what's
// asked and saved, the keys. The components named Brief… say which part a
// thing of it is drawn as, and the parts (MissionBrief…) are given what to
// draw.

// Wide enough for the brief and the plan side by side, each at a column
// worth reading: what the documents' pane has to be.
const sideBySide = 1120;

export function Brief({ d, root, updated, reload, onEscape }: { d: MissionDetail; root: string; updated?: string; reload: Reload; onEscape: () => void }) {
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
          label: acceptLabel({ briefing, kind: p.kind, unwritten: d.briefUnwritten }),
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
  const suggestions = suggestionsFor({ held: !!held, planner: !!planner, briefing });
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
      if (el && writing(el)) return el.blur();
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
  const roomy = useRoom(main, sideBySide);
  const side = wide && roomy && (planned || briefing);
  // The planner's pane: its conversation, or (briefed without one) where
  // one's asked for.
  const hasPlanner = !!planner || !ended;
  const open = beside && hasPlanner;
  // The toolbar starts where the documents' text does, however the panes
  // are laid out.
  const [tools, setTools] = useState<HTMLDivElement | null>(null);
  const inset = useInset(main, tools, wide, side, open);

  // Which document the bar's toolbar is for: the one last gone to, if it's
  // being edited; else the other.
  const editing = (["brief", "plan"] as const).filter((x) => stateOf(x)?.editing);
  const toolsFor = editing.includes(active) ? active : editing[0];

  // The plan's steps, as it reads now: the outline's list follows it as
  // it's typed, and a step ws has keeps where the mission stands on it.
  const stages = useMemo(() => planSteps(plan ? plan.text() : s.plan.text, d.stages), [plan, s.plan.text, d.stages]);
  const now = d.stages.find((x) => x.state === "current");

  const heldChip = held ? { ...passageChip(held), placeholder: "Ask about this passage…", drop: "Ask about the whole brief" } : undefined;
  const plannerField = canAsk ? (
    <BriefPlannerField held={heldChip} dropHeld={() => setHeld(undefined)} suggestions={suggestions} ask={ask} input={field} busy={asking} placeholder={planner ? "Ask the planner…" : "What should change in the plan, and why"} />
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
      <MissionBriefPlanner label="The planner's conversation" centre={centre} reading />
    ) : ended ? null : (
      <BriefNoPlanner briefing={briefing} busy={busy} briefIt={briefIt} field={plannerField} fold={wide ? () => fold(false) : undefined} />
    );

  // Nothing drafted: the conversation is the page, or where it's begun.
  if (!drafted) {
    return (
      <MissionBrief>
        <MissionBriefOutline>
          <MissionBriefOutlineSection title="The brief">
            <MissionBriefOutlineEmpty>Not drafted yet. Its planner writes it with you: the goal, the context, what's in and out, and how you'll know it's done.</MissionBriefOutlineEmpty>
          </MissionBriefOutlineSection>
          <MissionBriefOutlineSection title="The plan">
            <MissionBriefOutlineEmpty>Comes with the brief: what Z is, where it may stop early, and its steps.</MissionBriefOutlineEmpty>
          </MissionBriefOutlineSection>
        </MissionBriefOutline>
        <MissionBriefTalk>{planner ? plannerPane(true) : <MissionBriefStart request={d.request} busy={busy} onBrief={briefIt} onOwn={() => setOwn(true)} />}</MissionBriefTalk>
      </MissionBrief>
    );
  }

  const key = `${again}:${p?.n ?? "as-is"}`;
  // The plan is read, not typed in, until it's asked for: what's offered
  // of it is its planner writing it again from the brief.
  const guard: Guard = { rewrite: canRewrite ? rewritePlan : undefined, busy: asking || busy, planned };
  const panel = (id: DocId, title: string, doc: Doc, onState: (st: DocState) => void, empty?: string) => (
    <BriefDocPanel
      key={`${id}:${key}`}
      id={id}
      title={title}
      doc={stale ? { text: doc.text } : doc}
      empty={empty}
      guard={id === "plan" ? guard : undefined}
      readOnly={ended}
      side={side}
      focus={focus?.doc === id ? focus.id : undefined}
      onFocus={(cid) => setFocus({ doc: id, id: cid })}
      onState={onState}
      onSubmit={primary && !primary.disabled ? primary.run : undefined}
      toolbarIn={slot}
      toolbarOn={toolsFor === id}
      onActive={() => setActive(id)}
    />
  );
  // What the planner has for you while it's folded away: its turn to be
  // answered, or something it proposed that isn't the documents'.
  const plannerWaits = !!t && (following || (t.proposals ?? []).some((x) => x.state === "open" && x.kind !== "brief" && x.kind !== "plan"));

  return (
    <MissionBrief
      planner={open}
      bar={
        <MissionBriefBar>
          <BriefWhat ended={ended} briefing={briefing} proposal={p} draft={s.draft} changed={changed} confirmable={confirmable} now={now} steps={d.stages.length} />
          <MissionBriefToolbar
            ref={setTools}
            slot={setSlot}
            inset={inset}
            of={toolsFor && editing.length === 1 && toRead > 0 && (toolsFor === "plan" ? "The plan" : "The brief")}
            ofTitle="The other's changes are still to read: settled, it's yours to edit too"
            none={toolsFor ? undefined : ended ? "It's ended: nothing to edit" : toRead ? "Settle a document's changes, and it's yours to edit" : ""}
          />
          <MissionBriefActs>
            {busy && <Spinner />}
            {toRead > 0 && (
              <MissionBriefNav onBefore={() => move(-1)} onNext={() => move(1)}>
                {at >= 0 ? `${at + 1} of ${toRead}` : `${toRead} ${toRead === 1 ? "change" : "changes"}`}
              </MissionBriefNav>
            )}
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
                keys="mod+⏎"
                title={briefing && p ? "Nothing's written until you confirm it: your edits, and the changes you decline, go with it. The planner doesn't see them until then" : undefined}
              >
                {primary.label} <Keys k="mod+⏎" />
              </Button>
            )}
            {wide && !open && hasPlanner && <MissionBriefUnfold waits={plannerWaits} onUnfold={() => fold(true)} />}
          </MissionBriefActs>
        </MissionBriefBar>
      }
    >
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

      <MissionBriefMain ref={setMain}>
        <MissionBriefDocs ref={docs} side={side}>
          {stale && (
            <MissionBriefDocsNote tone="attention" title="Proposed against an earlier version" side={side}>
              It's changed since, so accepting this is refused. Ask the planner for it again, from what's written now.
            </MissionBriefDocsNote>
          )}
          {panel("brief", "The brief", s.brief, setBrief)}
          {(planned || briefing) && (
            <>
              {d.planProblem && (
                <MissionBriefDocsNote tone="attention" title="The plan doesn't read, so the mission works from its brief" side={side}>
                  {d.planProblem}
                </MissionBriefDocsNote>
              )}
              {panel("plan", "The plan", s.plan, setPlan, briefing ? (planner ? "No plan yet: its planner writes one with the brief, or from a brief of yours." : "No plan yet: its planner writes one with the brief.") : undefined)}
            </>
          )}
          {planProblem && (
            <MissionBriefDocsNote tone="danger" title="ws didn't save the plan" side={side}>
              <MissionBriefPlanProblem>{planProblem}</MissionBriefPlanProblem>
            </MissionBriefDocsNote>
          )}
          <BriefPlanMarks stages={d.stages} on={!briefing && !ended && !plan?.pending.length} />
          <BriefPassageTool within={docs} ask={askAbout} />
        </MissionBriefDocs>
        {wide && !open && canAsk && <MissionBriefAskFoot side={side}>{plannerField}</MissionBriefAskFoot>}
      </MissionBriefMain>

      {(open || !wide) && plannerPane()}
    </MissionBrief>
  );
}
