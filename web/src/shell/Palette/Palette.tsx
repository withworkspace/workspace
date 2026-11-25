import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { type ActionAsk, newPage, runPluginAction, startMissionAbout } from "../../api";
import { type FrameHere, useChromeStore } from "../../chrome/store";
import { useKeys } from "../../keys";
import { thingsNamed } from "../../refs/known";
import { isOutside, type Known, namedOf, type PageRef, type ThingAction } from "../../refs/refs";
import { forgetThing } from "../../refs/useThing";
import { Input } from "../../ui/Input";
import { humanError } from "../../ui/strings";
import { setTheme } from "../../ui/theme";
import { clock } from "../../ui/when";
import { toast } from "../../ui/toast";
import { useMedia } from "../../ui/useMedia";
import { usePlugins, useWorkspace } from "../../workspace/store";
import { actsFor, type Subject, subjectOf } from "../acts";
import { askShutDown } from "../daemon";
import {
  actingOn,
  type ActOn,
  askOptions,
  asks,
  build,
  type CodeEnd,
  type CodeHit,
  codeRematch,
  codeStream,
  findStream,
  type FoundGroup,
  type GroupKey,
  groupTitle,
  type Item,
  localItems,
  ordered,
  parseQuery,
  rematch,
  type Scope,
  scopePrefix,
  warmFind,
  withScope,
} from "../find";
import { pinNameOf } from "../pinNames";
import { useFocus } from "../useFocus";
import { around, hereAt, hereItems, keptAround, type Near, nearIds, nearItems } from "../here";
import type { Waiting } from "../notify";
import { doToThing } from "../perform";
import { loadMemory, noteChosen, noteVisit, recentItems, visitOf } from "../recent";
import { FindBar } from "./FindBar";
import { FindFoot } from "./FindFoot";
import { FindFootKey } from "./FindFootKey";
import { FindList } from "./FindList";
import { FindMore } from "./FindMore";
import { FindNone } from "./FindNone";
import { FindNote } from "./FindNote";
import { FindScopes } from "./FindScopes";
import { FindSection } from "./FindSection";
import { FindTag } from "./FindTag";
import { FindWindow } from "./FindWindow";
import { PalettePane } from "./PalettePane";
import { PaletteRow } from "./PaletteRow";

// Palette is Find (⌘K): one box that gets you to anything the workspace
// has, by what you remember of it. What the page holds (missions, pages,
// conversations, crew, things to do) is looked through here as you type;
// files, pull requests and what's typed read as a ref are asked of the
// server, which answers a kind at a time. What the files say (%) is its
// own scope, asked of the server's code search, tagged to the repo of the
// page Find was opened on until Backspace widens it.
//
// This is what Find knows and does: what's typed, what's found, where the
// cursor is, what a key or a press does. What it draws is the parts
// beside it, each given what to draw (Find*.tsx: the window, the bar and
// its tags, the scopes, the list in sections of rows, the foot, the pane
// and its cards); PaletteRow and PalettePane say which part a thing
// found is drawn as.

// What Find looks through, each with the letter that says it.
const scopes: { id: Scope; label: string; prefix?: string }[] = [
  { id: "all", label: "Everything" },
  { id: "pulls", label: "Pull requests", prefix: scopePrefix.pulls },
  { id: "files", label: "Files", prefix: scopePrefix.files },
  { id: "code", label: "Code", prefix: scopePrefix.code },
  { id: "do", label: "Do", prefix: scopePrefix.do },
];

// Thing is a plugin's thing, as a ref.
type Thing = Extract<PageRef, { kind: "thing" }>;

// thingOf is the plugin's thing a row is, if it's one: found by what it
// says, or named outright by what's typed.
function thingOf(it?: Item): Thing | undefined {
  if (it?.said?.thing === "thing" && it.said.card?.kind === "thing") return it.said.card;
  return it?.card?.kind === "thing" ? it.card : undefined;
}

// stale is how old the pull requests can be before Find says how old.
const stale = 10 * 60 * 1000;

// codeWait is how long code search waits for typing to pause before it
// asks: ripgrep isn't started for every letter.
const codeWait = 150;

export function Palette({
  waiting,
  start,
  pageRepo,
  path,
  search,
  frameHere,
  onClose,
}: {
  waiting: Waiting[];
  start?: string;
  pageRepo?: string;
  path: string; // where Find was opened: what's around it shows first
  search: string;
  frameHere?: FrameHere; // what a plugin's page said it shows beyond its address
  onClose: () => void;
}) {
  const workspace = useWorkspace();
  const plugins = usePlugins();
  const navigate = useNavigate();
  // How the plugins' things are named: what's typed may be one's name
  // (RAM-123), or where one is. Find sits outside what the pages name
  // things by, so it's worked out here, from the same plugins.
  // With them, what else the workspace says it has by name (its missions,
  // pages, conversations and artifacts, and the repos they're of): a
  // thing may point at any of those, and the pane draws what it points at.
  const named = JSON.stringify((plugins ?? []).map((p) => [p.name, p.refs ?? []]));
  const naming = [
    workspace.missions.map((m) => `${m.slug}:${m.capsules.map((c) => c.repo).join("+")}`).join(","),
    (workspace.pages ?? []).map((p) => p.id).join(","),
    workspace.conversations.map((t) => t.id).join(","),
    (workspace.artifacts ?? []).map((a) => a.id).join(","),
    (workspace.spaces ?? []).map((s) => s.repo).join(","),
  ].join("|");
  const known = useMemo<Known>(
    () => ({
      repos: new Map([...workspace.missions.flatMap((m) => m.capsules.map((c) => c.repo)), ...(workspace.spaces ?? []).map((s) => s.repo)].map((r) => [r, undefined])),
      missions: workspace.missions.map((m) => m.slug),
      pages: (workspace.pages ?? []).map((p) => p.id),
      talks: workspace.conversations.map((t) => t.id),
      artifacts: (workspace.artifacts ?? []).map((a) => a.id),
      things: thingsNamed(plugins ?? []),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [named, naming],
  );
  // Acting on a thing (⌘K on its row), a plugin's or one of ws's own:
  // what can be done to it is the list, until one's chosen or it's left.
  const [acting, setActing] = useState<{ thing?: Thing; subject?: Subject; title: string; typed: string }>();
  // An action that's asked something before it goes on: of a thing, or a
  // plugin's own. What it's been answered so far is sent again with each
  // answer, until it asks nothing more.
  const [asking, setAsking] = useState<{ plugin: string; action: ThingAction; thing?: Thing; answers: string[]; ask: ActionAsk; typed: string }>();
  const [working, setWorking] = useState(false);
  const [raw, setRaw] = useState(start ?? "");
  const [only, setOnly] = useState<GroupKey>(); // narrowed with ⇥ to a kind the page holds
  const [cursor, setCursor] = useState<string>(); // the id of the row the cursor's on
  const [found, setFound] = useState<{
    q: string;
    groups: FoundGroup[];
    done: boolean;
  }>({ q: "", groups: [], done: true });
  // Code search's tag: the repo it's narrowed to (the page's, to start).
  const [tag, setTag] = useState(pageRepo);
  const [code, setCode] = useState<{ key: string; hits: CodeHit[]; end?: CodeEnd; done: boolean }>({ key: "", hits: [], done: true });
  const list = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  // Whether there's room for a pane beside the list.
  const wide = useMedia("(min-width: 1100px)");

  const { scope, q } = parseQuery(raw, workspace.root);
  const group = scope === "all" ? only : scope;
  // What the page holds, with what the plugins do of their own.
  const does = JSON.stringify((plugins ?? []).map((p) => [p.name, p.actions ?? []]));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const local = useMemo(() => localItems(workspace, plugins), [workspace, does]);
  const memory = useMemo(() => loadMemory(workspace.root), [workspace.root]);

  // Opening Find has the server read what's due, before the first letter.
  useEffect(warmFind, []);

  // Ask the server for what the page doesn't hold. A new letter drops the
  // last request.
  const asked = asks(scope, only, q);
  const kinds = asked?.kinds?.join(",");
  const limit = asked?.limit;
  useEffect(() => {
    if (limit === undefined) {
      setFound({ q, groups: [], done: true });
      return;
    }
    const drop = new AbortController();
    // Until it answers: what the last answer had that still matches.
    setFound((was) => ({
      q,
      groups: q && was.q && q.startsWith(was.q) ? rematch(was.groups, q) : [],
      done: false,
    }));
    const got: FoundGroup[] = [];
    findStream(q, kinds?.split(","), limit, drop.signal, (g) => {
      got.push(g);
      setFound((was) => ({
        q,
        done: false,
        groups: [...was.groups.filter((x) => x.kind !== g.kind), g],
      }));
    }).then(
      () => !drop.signal.aborted && setFound({ q, groups: got, done: true }),
      () => !drop.signal.aborted && setFound({ q, groups: got, done: true }),
    );
    return () => drop.abort();
  }, [q, kinds, limit]);

  // Code search: once typing pauses, streamed. A new letter (or the tag
  // cleared) drops the last request, and the server stops ripgrep.
  const codeQ = scope === "code" ? q : "";
  useEffect(() => {
    const key = `${tag ?? ""}\n${codeQ}`;
    if (!codeQ) {
      setCode({ key, hits: [], done: true });
      return;
    }
    const drop = new AbortController();
    // Until it answers: what was found a moment ago that still says it.
    setCode((was) => ({ key, hits: was.key.startsWith(`${tag ?? ""}\n`) ? codeRematch(was.hits, codeQ) : [], end: undefined, done: false }));
    const wait = setTimeout(() => {
      const hits: CodeHit[] = [];
      let frame = 0;
      // Hits are shown a frame at a time, not one render each.
      const show = () => {
        frame = 0;
        if (!drop.signal.aborted) setCode({ key, hits: [...hits], done: false });
      };
      codeStream([codeQ], { repo: tag }, drop.signal, (h) => {
        hits.push(h);
        if (!frame) frame = requestAnimationFrame(show);
      }).then(
        (end) => {
          cancelAnimationFrame(frame);
          if (!drop.signal.aborted) setCode({ key, hits, end, done: true });
        },
        (e: Error) => {
          cancelAnimationFrame(frame);
          if (!drop.signal.aborted) setCode({ key, hits, end: { matches: hits.length, bytes: 0, problem: humanError(e.message) }, done: true });
        },
      );
    }, codeWait);
    return () => {
      clearTimeout(wait);
      drop.abort();
    };
  }, [codeQ, tag]);

  const needs = useMemo(() => {
    const byId = new Map(local.map((it) => [it.to, it]));
    return waiting.flatMap((w) => {
      const it = byId.get(w.path);
      return it ? [{ ...it, group: "needs" as const, hint: w.body }] : [];
    });
  }, [waiting, local]);
  const recent = useMemo(() => recentItems(memory.visits, local), [memory, local]);

  // What the page is of, what can be done to it, and what's one link from
  // it: as last answered for it, at once, then as answered now.
  const here = useMemo(() => hereAt(path, search, workspace, known, frameHere), [path, search, workspace, known, frameHere]);
  const [near, setNear] = useState<Near[]>(() => (here ? keptAround(here) : []));
  const hereKey = here && `${here.scope ?? ""}\n${here.ref}`;
  useEffect(() => {
    // A place in Settings has things to do there, and nothing linked.
    if (!here || here.id.startsWith("settings:")) return;
    const drop = new AbortController();
    // At once as the records stand, then as they are once read again.
    // What can't be asked is said by saying nothing.
    around(here, false, drop.signal)
      .then(setNear)
      .then(() => around(here, true, drop.signal))
      .then(setNear, () => undefined);
    return () => drop.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hereKey]);
  const doHere = useMemo(() => hereItems(here, workspace, known), [here, workspace, known]);
  const linked = useMemo(() => (here ? nearItems(near, here, local, workspace.spaces) : []), [near, here, local, workspace.spaces]);
  const nearby = useMemo(() => nearIds(linked, near), [linked, near]);

  const built = useMemo(
    () =>
      build({
        q,
        scope,
        only,
        local,
        found: found.groups,
        needs,
        recent,
        at: here?.id,
        hereTitle: here?.title,
        here: doHere,
        linked,
        near: nearby,
        memory,
        spaces: workspace.spaces,
        known,
        code: scope === "code" ? code : undefined,
      }),
    [q, scope, only, local, found.groups, needs, recent, here, doHere, linked, nearby, memory, workspace.spaces, known, code],
  );
  // What the list is: what an action asks to pick from, what can be done
  // to the thing Find's acting on, or, most of the time, what's found.
  const mode = asking ? "asking" : acting ? "acting" : undefined;
  const typed = asking ? asking.typed : acting ? acting.typed : raw;
  const rows = asking
    ? askOptions(asking.ask, asking.typed)
    : acting?.thing
      ? actingOn(namedOf(acting.thing, known), acting.typed, acting.thing.id)
      : acting?.subject
        ? ordered(actsFor(acting.subject, workspace), acting.typed)
        : built.rows;
  // Until the cursor's moved, it's on the first row that goes somewhere:
  // what can be done to where you are is a step down, so ⌘K then ⏎ never
  // does it by reflex.
  const chosen = rows.findIndex((r) => r.id === cursor);
  const at = chosen >= 0 ? chosen : mode ? 0 : rows.findIndex((r) => r.group !== "here");
  const on = at >= 0 ? rows[at] : undefined;
  // The plugin's thing the cursor's on, if it's on one: found by what it
  // says, or named outright by what's typed. While Find acts on one, or
  // one's action asks, it's that one.
  const onThing = mode ? (asking?.thing ?? acting?.thing) : thingOf(on);
  // One of ws's own things the cursor's on, with something to do to it.
  const onSubject = mode ? undefined : subjectOf(on?.id);
  const actable = !!onSubject && actsFor(onSubject, workspace).length > 0;
  // ⌘. pins what the row is, or unpins it: anything with a card.
  const focus = useFocus();
  const pinRef = mode ? undefined : (onThing ?? on?.card);
  const pinName = pinRef && focus?.supported ? pinNameOf(pinRef) : undefined;
  const pinnedNow = !!pinName && !!focus?.pinned(pinName);

  useEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [on?.id]);

  const move = (d: number) => {
    if (!rows.length) return;
    setCursor(rows[(Math.max(at, d > 0 ? -1 : 0) + d + rows.length) % rows.length].id);
  };

  // run runs a plugin's action: one of its own, or, with a thing, one done
  // to it. What it asks first, Find asks, and runs it again with the
  // answer. Done, it says what the plugin said, goes where it said, and
  // closes; what went wrong is said, and Find stays where it was.
  const run = (plugin: string, action: ThingAction, thing: Thing | undefined, answers: string[]) => {
    if (working) return;
    setWorking(true);
    runPluginAction(plugin, action.id, { kind: thing?.what, ref: thing?.id, answers })
      .then(
        (res) => {
          if (res.ask) {
            setAsking({ plugin, action, thing, answers, ask: res.ask, typed: "" });
            setCursor(undefined);
            return;
          }
          // What's kept of the thing is as it was before: it's read again.
          if (thing) forgetThing(thing.plugin, thing.what, thing.id);
          if (res.say) toast(res.say, { tone: "done" });
          onClose();
          if (res.open) navigate(res.open);
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => {
        setWorking(false);
        input.current?.focus(); // what it asks next is typed, or picked, from here
      });
  };

  // doTo does something to a plugin's thing: one of its kind's actions,
  // which its plugin does, or ws's own.
  const doTo = (thing: Thing, a: ActOn) => {
    if (!a.ws) {
      run(thing.plugin, { id: a.id, title: a.title }, thing, []);
      return;
    }
    if (working) return;
    setWorking(true);
    startMissionAbout(thing.id)
      .then(
        (m) => {
          onClose();
          navigate(m.path);
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setWorking(false));
  };

  // back steps out of what Find's in the middle of: an ask, to the one
  // before it (the action's run again with one answer fewer) or, from its
  // first, to where it was asked from; and acting on a thing, to what was
  // typed.
  const back = () => {
    setCursor(undefined);
    if (asking) {
      if (asking.answers.length > 0) run(asking.plugin, asking.action, asking.thing, asking.answers.slice(0, -1));
      else setAsking(undefined);
      return;
    }
    setActing(undefined);
  };

  // choose is Enter, or a click, on a row of an ask or of what can be
  // done to a thing; and Enter on a line an ask wants typed.
  const choose = (it: Item | undefined) => {
    if (asking) {
      const answer = asking.ask.type === "pick" ? it?.option : asking.typed;
      if (answer === undefined) return;
      run(asking.plugin, asking.action, asking.thing, [...asking.answers, answer]);
      return;
    }
    if (acting?.thing && it?.act) doTo(acting.thing, it.act);
    else if (acting?.subject && it) perform(it, false);
  };

  const go = (it: Item | undefined, tab: boolean) => {
    if (mode) {
      choose(it);
      return;
    }
    if (!it) return;
    const visit = visitOf(it, Date.now());
    if (visit) noteVisit(workspace.root, visit);
    noteChosen(workspace.root, q, it.id);
    perform(it, tab);
  };

  // perform does what a row does: goes where it goes, or does what it
  // does.
  const perform = (it: Item, tab: boolean) => {
    if (it.doing && "does" in it.doing) {
      // Done to the thing the page is of: Find stays, for what it may ask.
      doTo(it.doing.thing, it.doing.does);
      return;
    }
    if (it.doing && "plugin" in it.doing) {
      // A plugin's own: Find stays, for what it may ask first.
      run(it.doing.plugin, it.doing.action, undefined, []);
      return;
    }
    if (it.doing) {
      onClose();
      const d = it.doing;
      if ("theme" in d) setTheme(d.theme);
      else if ("help" in d) useChromeStore.getState().setHelp(true);
      else if ("shutDown" in d) void askShutDown();
      else if ("page" in d)
        newPage().then(
          (p) => navigate(`/documents/local/${encodeURIComponent(p.id)}`),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
      else doToThing(d);
      return;
    }
    if (!it.to) return;
    // Outside ws (a plugin's thing with nowhere to open here): a tab of
    // its own, whichever way it was chosen.
    if (tab || isOutside(it.to)) {
      window.open(it.to, "_blank", "noopener");
      if (!tab) onClose();
      return;
    }
    onClose();
    navigate(it.to);
  };

  // ⇥ narrows to the group the cursor's in, and again goes back.
  const narrow = () => {
    if (group) {
      setOnly(undefined);
      setRaw(withScope(raw, "all", workspace.root));
      return;
    }
    const g = built.sections.find((s) => s.items.some((it) => it.id === on?.id))?.group;
    // The best line, what needs you and where you've been aren't kinds:
    // narrow to the kind of thing the row is.
    const kind = g === "best" || g === "here" || g === "needs" || g === "linked" || g === "recent" ? kindOf(on) : g;
    if (!kind) return;
    if (kind === "pulls" || kind === "files" || kind === "do") setRaw(withScope(raw, kind, workspace.root));
    else setOnly(kind);
    setCursor(undefined);
  };

  const copy = () => {
    if (!on?.ref) return;
    navigator.clipboard?.writeText(on.ref).then(() => toast(`Copied ${on.ref}`));
  };

  // Escape, from where it's not typed into: a step back out of what Find's
  // in the middle of, and Find closed from where it finds.
  useKeys({ Escape: mode ? back : onClose }, { modal: true });

  const pulls = found.groups.find((g) => g.kind === "pulls");
  const pullsAge = pulls?.at ? Date.now() - new Date(pulls.at).getTime() : 0;
  const busy = working || (mode ? false : scope === "code" ? !code.done : !found.done);
  const nothing = !mode && !rows.length && (scope === "code" ? code.done : found.done || !asked);
  // What it's in the middle of, said before what's typed: the thing it's
  // acting on, and the action that's asking.
  const doing = asking ? `${asking.thing ? `${asking.thing.id}: ` : ""}${asking.action.title}` : acting ? acting.title : undefined;
  const placeholder = asking
    ? asking.ask.placeholder || (asking.ask.type === "pick" ? `${asking.ask.title}…` : `${asking.ask.title}: type it, then ⏎`)
    : acting
      ? `Do something to ${acting.title}…`
      : group === "pulls"
      ? "A number, a title, who it's by…"
      : group === "files"
        ? "A few letters of a path, and :12 for a line…"
        : group === "code"
          ? tag
            ? `Text in ${tag}'s files…`
            : "Text in the files, across the workspace…"
          : group === "do"
          ? "Something to do…"
          : group
            ? `Find in ${groupTitle[group].toLowerCase()}…`
            : "Find anything: a mission, a file, a pull request, an address…";
  // What's typed is the mode's own while Find's in one, and what's being
  // found stays as it was, to come back to.
  const type = (text: string) => {
    if (asking) setAsking({ ...asking, typed: text });
    else if (acting) setActing({ ...acting, typed: text });
    else setRaw(text);
    setCursor(undefined);
  };
  const over = (it: Item) => () => it.id !== on?.id && setCursor(it.id);

  return (
    <FindWindow
      onClose={onClose}
      pane={wide && <PalettePane on={on} thing={onThing} finding={!mode} known={known} working={working} onDo={doTo} onClose={onClose} />}
    >
      <FindBar
        busy={busy}
        tags={
          <>
            {doing && (
              <FindTag doing onClick={back} title="Back" keys="esc">
                {doing}
              </FindTag>
            )}
            {!mode && only && scope === "all" && (
              <FindTag onClick={() => setOnly(undefined)} title="Look through everything again">
                {groupTitle[only]}
              </FindTag>
            )}
            {!mode && scope === "code" && tag && (
              <FindTag onClick={() => setTag(undefined)} title="Search the whole workspace" keys="⌫">
                repo: {tag}
              </FindTag>
            )}
          </>
        }
      >
        <Input
          ref={input}
          autoFocus
          value={typed}
          placeholder={placeholder}
          aria-label="Find"
          role="combobox"
          aria-expanded="true"
          aria-controls="palette-list"
          aria-activedescendant={on ? `pal-${on.id}` : undefined}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => type(e.target.value)}
          onKeyDown={(e) => {
            const mod = e.metaKey || e.ctrlKey;
            if (e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")) {
              e.preventDefault();
              move(1);
            } else if (e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")) {
              e.preventDefault();
              move(-1);
            } else if (e.key === "Enter") {
              e.preventDefault();
              go(on, mod);
            } else if (mod && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "k") {
              // ⌘K, with Find open: act on the thing the cursor's on, a
              // plugin's or ws's own. What can be done to it is the list.
              e.preventDefault();
              if (!mode && onThing) {
                setActing({ thing: onThing, title: onThing.id, typed: "" });
                setCursor(undefined);
              } else if (!mode && actable && on) {
                setActing({ subject: onSubject, title: on.title, typed: "" });
                setCursor(undefined);
              }
            } else if (mod && e.key === "." && pinName && focus) {
              e.preventDefault();
              if (pinnedNow) focus.unpin(pinName);
              else focus.pin(pinName);
            } else if (mode && (e.key === "Escape" || (e.key === "Backspace" && !typed))) {
              // Out of what it's in the middle of, a step at a time:
              // Escape closes Find only from where it finds.
              e.preventDefault();
              back();
            } else if (mode) {
              // The rest are for finding: ⇥ narrows nothing here. (It's
              // stopped, too: the modal would move the focus on.)
              if (e.key === "Tab") {
                e.preventDefault();
                e.stopPropagation();
              }
            } else if (e.key === "Tab" && !e.shiftKey) {
              // ⇥ is Find's: stopped, or the modal moves the focus on to
              // the next thing in it.
              e.preventDefault();
              e.stopPropagation();
              narrow();
            } else if (e.key === "Backspace" && !raw && only) {
              e.preventDefault();
              setOnly(undefined);
            } else if (e.key === "Backspace" && scope === "code" && !q && tag && e.currentTarget.selectionStart === raw.length) {
              // Nothing typed after %: the tag goes first, and the search
              // widens to the workspace. Again, and % goes.
              e.preventDefault();
              setTag(undefined);
            } else if (mod && e.key.toLowerCase() === "c" && e.currentTarget.selectionStart === e.currentTarget.selectionEnd && on?.ref) {
              // Nothing of what's typed is selected: copy the row's name.
              e.preventDefault();
              copy();
            }
          }}
        />
      </FindBar>
      {!mode && (
        <FindScopes
          scopes={scopes}
          value={scope === "all" && only ? undefined : scope}
          onPick={(to) => {
            setOnly(undefined);
            setRaw(withScope(raw, to, workspace.root));
            setCursor(undefined);
          }}
        />
      )}
      <FindList id="palette-list" ref={list} keepFocus={!!mode}>
        {mode && (
          <FindSection label={asking ? asking.ask.title : "What can be done to it"} title={asking ? asking.ask.title : `Do to ${acting?.title}`}>
            {rows.map((it) => (
              <PaletteRow key={it.id} it={it} on={it.id === on?.id} section="do" onOver={over(it)} onPick={() => choose(it)} />
            ))}
            {asking?.ask.type === "text" && <FindNote>Type it, then ⏎ sends it.</FindNote>}
            {!rows.length && asking?.ask.type !== "text" && <FindNone>{typed ? `Nothing here matches “${typed}”.` : "Nothing here."}</FindNone>}
          </FindSection>
        )}
        {!mode &&
          built.sections.map((s) => (
            <FindSection key={s.group} label={s.title} title={built.sections.length > 1 || s.group !== group ? s.title : undefined}>
              {s.items.map((it) => (
                <PaletteRow key={it.id} it={it} on={it.id === on?.id} section={s.group} onOver={over(it)} onPick={(tab) => go(it, tab)} />
              ))}
              {s.more > 0 && <FindMore count={s.more} narrows={!group && s.group !== "best"} />}
              {s.note && <FindNote capped={s.group === "code" && !!code.end?.capped}>{s.note}</FindNote>}
            </FindSection>
          ))}
        {nothing && (
          <FindNone>
            {scope === "code"
              ? q
                ? `No file ${tag ? `in ${tag} ` : ""}says “${q}”.`
                : `Type text to find in the files of ${tag ?? "every repo, and what capsules have changed"}.`
              : q
                ? `Nothing matches “${q}”.`
                : group === "pulls" && !pulls?.at
                  ? "Pull requests show here once Reviews has been opened."
                  : group === "files"
                    ? "No capsule has changed anything. Type a few letters of a path."
                    : "Nothing here."}
          </FindNone>
        )}
      </FindList>
      {mode ? (
        <FindFoot>
          <FindFootKey k="⏎">{asking ? (asking.ask.type === "pick" ? "pick it" : "send it") : "do it"}</FindFootKey>
          <FindFootKey k="esc">back</FindFootKey>
        </FindFoot>
      ) : (
        <FindFoot aside={pulls?.at && pullsAge > stale ? `Pull requests as of ${clock(pulls.at)}` : undefined}>
          <FindFootKey k="⏎">go</FindFootKey>
          <FindFootKey k="mod+⏎">new tab</FindFootKey>
          <FindFootKey k="⇥">{group ? "everything" : "only these"}</FindFootKey>
          {scope === "code" && tag && !q && <FindFootKey k="⌫">the whole workspace</FindFootKey>}
          {(onThing || actable) && <FindFootKey k="mod+K">act on it</FindFootKey>}
          {on?.ref && <FindFootKey k="mod+C">copy its name</FindFootKey>}
          {pinName && <FindFootKey k="mod+.">{pinnedNow ? "unpin it" : "pin it"}</FindFootKey>}
        </FindFoot>
      )}
    </FindWindow>
  );
}

// kindOf is the kind of thing an item is, whichever group it's shown in.
function kindOf(it?: Item): GroupKey | undefined {
  if (!it) return undefined;
  const k = it.id.slice(0, it.id.indexOf(":"));
  return (
    {
      code: "code",
      mission: "missions",
      pr: "pulls",
      file: "files",
      dir: "files",
      page: "pages",
      talk: "talks",
      crew: "crew",
      capsule: "capsules",
      do: "do",
      place: "places",
      document: "documents",
      said: "said",
    } as Record<string, GroupKey>
  )[k];
}
