import { type MouseEvent, type RefObject, useEffect, useState } from "react";
import type { Stage } from "../api";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Keys } from "../ui/kit";
import { type Edit, editsBySection } from "./briefpage";
import { PlanList } from "./PlanList";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";

// The outline beside a mission's brief and plan (BriefTab.tsx): what
// they're made of, each leading to its place in them.

export type DocId = "brief" | "plan";

// Reading is a document's changes still to settle, and the text they're
// marked in.
export interface Reading {
  pending: Edit[];
  base: string;
}

interface Heading {
  doc: DocId;
  text: string;
  el: HTMLElement;
}

// The step a plan's nth is, in the plan as it's drawn: its first numbered
// list is its steps.
export const stepIn = (docs: HTMLElement | null, n: number) => docs?.querySelector<HTMLElement>(`.brief-doc[data-doc="plan"] .markdown > ol:first-of-type > li:nth-child(${n})`) ?? null;

// BriefOutline is the changes still to read (stepped through, or all
// taken), then the brief's sections and the plan's, each with how many
// changes are to read under it, and the plan's steps as the mission's
// details list them (stages: as the plan reads now, so they follow it as
// it's typed). The section being read is marked. docs is the documents'
// element: their headings are read from it as they're drawn, so the list
// is theirs whether they're being read as changes or edited.
export function BriefOutline({
  docs,
  briefing,
  brief,
  plan,
  stages,
  planned,
  fresh,
  at,
  move,
  focus,
  acceptAll,
  now,
  request,
}: {
  docs: RefObject<HTMLElement | null>;
  briefing: boolean;
  brief?: Reading;
  plan?: Reading;
  stages: Stage[];
  planned: boolean;
  fresh: boolean; // the plan's new with this draft
  at: number; // which change is in hand, of them all; -1 for none
  move: (by: number) => void;
  focus: (doc: DocId, id: string) => void;
  acceptAll: () => void;
  now?: Stage; // the step the mission's at
  request?: string;
}) {
  const [heads, setHeads] = useState<Heading[]>([]);
  const [here, setHere] = useState<Set<HTMLElement>>(new Set());

  // The documents' headings, as they're drawn: read again as they change.
  // (An effect, not a layout effect: the documents' element comes after
  // this one, and its ref isn't set until the layout's done.)
  useEffect(() => {
    const el = docs.current;
    if (!el) return;
    let timer = 0;
    const look = () => {
      const found: Heading[] = [];
      for (const doc of el.querySelectorAll<HTMLElement>(".brief-doc")) {
        for (const h of doc.querySelectorAll<HTMLElement>(".markdown :is(h1, h2, h3)")) found.push({ doc: doc.dataset.doc as DocId, text: h.textContent?.trim() ?? "", el: h });
      }
      setHeads((was) => (was.length === found.length && was.every((h, i) => h.el === found[i].el && h.text === found[i].text) ? was : found));
    };
    look();
    // (The documents draw themselves after this: looked at again once they have.)
    const first = requestAnimationFrame(look);
    const seen = new MutationObserver(() => {
      clearTimeout(timer);
      timer = window.setTimeout(look, 40);
    });
    seen.observe(el, { childList: true, subtree: true, characterData: true });
    return () => {
      seen.disconnect();
      clearTimeout(timer);
      cancelAnimationFrame(first);
    };
  }, [docs]);

  // Which is being read: the last heading above a line near the top, in
  // each thing that scrolls (the page, the documents together, or each by
  // itself).
  useEffect(() => {
    const el = docs.current;
    if (!el) return;
    const main = document.querySelector<HTMLElement>("main");
    const scrolls = (x: HTMLElement) => x.scrollHeight > x.clientHeight + 1 && getComputedStyle(x).overflowY !== "visible";
    const look = () => {
      const panes = [...el.querySelectorAll<HTMLElement>(".brief-doc")].filter(scrolls);
      const scrollers = panes.length ? panes : [scrolls(el) || !main ? el : main];
      const read = new Set<HTMLElement>();
      for (const sc of scrollers) {
        const line = Math.max(sc.getBoundingClientRect().top, 0) + 110;
        const mine = heads.filter((h) => sc.contains(h.el));
        let cur: HTMLElement | undefined;
        for (const h of mine) if (h.el.getBoundingClientRect().top <= line) cur = h.el;
        cur ??= mine[0]?.el;
        if (cur) read.add(cur);
      }
      setHere((was) => (was.size === read.size && [...read].every((x) => was.has(x)) ? was : read));
    };
    look();
    const on = [el, ...el.querySelectorAll<HTMLElement>(".brief-doc"), ...(main ? [main] : [])];
    for (const sc of on) sc.addEventListener("scroll", look, { passive: true });
    window.addEventListener("resize", look);
    return () => {
      for (const sc of on) sc.removeEventListener("scroll", look);
      window.removeEventListener("resize", look);
    };
  }, [docs, heads]);

  const goTo = (el: HTMLElement) => el.scrollIntoView({ block: "start", behavior: "smooth" });
  const waiting = { brief: editsBySection(brief?.base ?? "", brief?.pending ?? []), plan: editsBySection(plan?.base ?? "", plan?.pending ?? []) };
  const toRead = (brief?.pending.length ?? 0) + (plan?.pending.length ?? 0);
  const row = (h: Heading) => {
    const list = waiting[h.doc].get(h.text) ?? [];
    return (
      <li key={`${h.doc}:${h.text}`}>
        <button type="button" className={`map-item brief-item${here.has(h.el) ? " here" : ""}`} aria-current={here.has(h.el) || undefined} onClick={() => (list.length ? focus(h.doc, list[0].id) : goTo(h.el))}>
          <span>{h.text}</span>
          {list.length > 0 && (
            <Count tone="attention" className="outline-tag" title={`${list.length} to read here`}>
              {list.length}
            </Count>
          )}
        </button>
      </li>
    );
  };
  // A step of the list goes to it in the plan, which marks it a moment.
  const toStep = (e: MouseEvent) => {
    const n = Number((e.target as Element).closest("li.step")?.querySelector(".step-n")?.textContent);
    const el = n ? stepIn(docs.current, n) : null;
    if (!el) return;
    goTo(el);
    el.classList.remove("brief-flash");
    void el.offsetWidth;
    el.classList.add("brief-flash");
  };
  const of = (doc: DocId) => heads.filter((h) => h.doc === doc);
  return (
    <aside className="brief-outline" aria-label="What the brief and plan are made of">
      {toRead > 0 && (
        <section className="brief-changes">
          <h3>
            To read
            <Count tone="attention" className="outline-tag">{toRead}</Count>
            <span className="spacer" />
            {at >= 0 && (
              <span className="brief-changes-at">
                {at + 1} of {toRead}
              </span>
            )}
          </h3>
          <p>
            {toRead === 1 ? "A change" : `${toRead} changes`} {briefing ? "since the draft before" : "to what's written"}
          </p>
          <div className="brief-changes-nav">
            <Button size="small" className="changes-nav-button" type="button" onClick={() => move(-1)} title="The change before (k)" aria-label="The change before">
              <Icon name="chevron-up" size={12} /> <Keys k="k" />
            </Button>
            <Button size="small" className="changes-nav-button" type="button" onClick={() => move(1)} title="The next change (j)" aria-label="The next change">
              <Icon name="chevron-down" size={12} /> <Keys k="j" />
            </Button>
            <span className="spacer" />
            <Button size="small" className="changes-nav-button" type="button" onClick={acceptAll} title="Take each as it's proposed">
              Accept all
            </Button>
          </div>
        </section>
      )}
      {!toRead && now && (
        <section className="brief-at">
          <h3>Now</h3>
          <p>
            Step {now.n} of {stages.length}: {now.title}
          </p>
        </section>
      )}
      <section>
        <h3>The brief</h3>
        {of("brief").length ? <ul>{of("brief").map(row)}</ul> : <p className="map-empty">No sections yet.</p>}
      </section>
      <section>
        <h3>
          The plan
          {fresh && <Chip tone="quiet" className="outline-tag">new</Chip>}
        </h3>
        {planned ? (
          <>
            {of("plan").length > 0 && <ul>{of("plan").map(row)}</ul>}
            {stages.length > 0 && (
              // (Keys go to a step by the plan itself: the list is the pointer's.)
              <div className="brief-steps" onClick={toStep}>
                <PlanList stages={stages} />
              </div>
            )}
          </>
        ) : (
          <p className="map-empty">{briefing ? "No plan yet: its planner writes one with the brief." : "No plan: it works from its brief's stages."}</p>
        )}
      </section>
      {request && (
        <section className="brief-requested">
          <h3>Requested</h3>
          <p title={request}>
            <Inline text={request} />
          </p>
        </section>
      )}
    </aside>
  );
}
