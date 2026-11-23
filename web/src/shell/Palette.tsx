import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { act, type ActionAsk, dockPR, newPage, restoreArtifact, runPluginAction, setTalkStatus, startMissionAbout } from "../api";
import { useKeys } from "../keys";
import { archiveMission } from "../mission/archive";
import { RefsScope, thingsNamed } from "../refs/known";
import { RefCardBody, ThingCard } from "../refs/RefCard";
import { isOutside, type Known, namedOf, type PageRef, refAt, refKey, type ThingAction } from "../refs/refs";
import { RefTag } from "../refs/RefTag";
import { forgetThing } from "../refs/ThingView";
import { marked } from "../ui/fuzzy";
import { FileIcon } from "../ui/FileIcon";
import { Icon } from "../ui/Icon";
import { Status } from "../ui/status";
import { confirm, humanError, Keys, toast } from "../ui/kit";
import { setTheme } from "../ui/theme";
import { clock } from "../ui/time";
import {
  actIcon,
  actingOn,
  type ActOn,
  actsOn,
  askOptions,
  asks,
  build,
  type CodeEnd,
  type CodeHit,
  codeRematch,
  codeStream,
  type Found,
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
} from "./find";
import type { Waiting } from "./notify";
import { CodeText, CodeWhere } from "./CodeHitParts";
import type { FrameHere } from "../plugins/here";
import { around, hereAt, hereItems, keptAround, type Near, nearIds, nearItems } from "./here";
import { loadMemory, noteChosen, noteVisit, recentItems, visitOf } from "./recent";
import { PinButton, pinNameOf, useFocus } from "./focus";
import { actsFor, type Subject, subjectOf, type WsDoing } from "./acts";
import { Chip } from "../ui/Chip";
import { Button } from "../ui/Button";
import { capital } from "../ui/text";
import { usePlugins, useWorkspace } from "../workspace/store";

// Palette is Find (⌘K): one box that gets you to anything the workspace
// has, by what you remember of it. What the page holds (missions, pages,
// conversations, crew, things to do) is looked through here as you type;
// files, pull requests and what's typed read as a ref are asked of the
// server, which answers a kind at a time. What the files say (%) is its
// own scope, asked of the server's code search, tagged to the repo of the
// page Find was opened on until Backspace widens it.

const chips: { scope: Scope; label: string }[] = [
  { scope: "all", label: "Everything" },
  { scope: "pulls", label: "Pull requests" },
  { scope: "files", label: "Files" },
  { scope: "code", label: "Code" },
  { scope: "do", label: "Do" },
];

// Thing is a plugin's thing, as a ref.
type Thing = Extract<PageRef, { kind: "thing" }>;

// thingOf is the plugin's thing a row is, if it's one: found by what it
// says, or named outright by what's typed.
function thingOf(it?: Item): Thing | undefined {
  if (it?.said?.thing === "thing" && it.said.card?.kind === "thing") return it.said.card;
  return it?.card?.kind === "thing" ? it.card : undefined;
}

// stopCrew stops a crew, once you've said so, as its own page asks.
async function stopCrew(slug: string, crew: string) {
  const yes = await confirm({
    title: `Stop ${crew}?`,
    body: "Its session can be resumed later if it recorded one: what it's in the middle of is lost.",
    confirm: `Stop ${crew}`,
    danger: true,
  });
  return yes ? act(slug, "stop", { crew }) : undefined;
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
  onHelp,
  onShutDown,
}: {
  waiting: Waiting[];
  start?: string;
  pageRepo?: string;
  path: string; // where Find was opened: what's around it shows first
  search: string;
  frameHere?: FrameHere; // what a plugin's page said it shows beyond its address
  onClose: () => void;
  onHelp: () => void;
  onShutDown: () => void;
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
  const wide = useWide();

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
    list.current?.querySelector(".on")?.scrollIntoView({ block: "nearest" });
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

  // doWs does what ws does to one of its things that isn't a mission's
  // act, and says what it did.
  const doWs = (d: WsDoing): Promise<string> =>
    "talk" in d
      ? setTalkStatus("", d.talk, d.status).then((t) => (t.status === "resolved" ? "Resolved." : "Reopened."))
      : "restore" in d
        ? restoreArtifact(d.restore).then(() => "Restored.")
        : dockPR(d.dock).then((r) => r.message);

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
      else if ("help" in d) onHelp();
      else if ("shutDown" in d) onShutDown();
      else if ("ws" in d)
        doWs(d.ws).then(
          (said) => toast(said, { tone: "done" }),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
      else if ("page" in d)
        newPage().then(
          (p) => navigate(`/documents/local/${encodeURIComponent(p.id)}`),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
      else
        (d.act === "archive" ? archiveMission(d.slug) : d.act === "stop" ? stopCrew(d.slug, d.crew ?? "") : act(d.slug, d.act, { crew: d.crew })).then(
          (r) => r && toast(r.message, { tone: "done" }),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
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
  // A name one of a thing's fields holds, in the pane: as what it names,
  // where ws reads it as a name, and its words where it doesn't.
  const pointer = (name: string) => {
    const ref = refAt(name, known);
    return ref ? (
      <RefTag refKey={refKey(ref)} inline>
        {name}
      </RefTag>
    ) : (
      name
    );
  };

  return (
    <div className="overlay palette-overlay" onMouseDown={onClose}>
      <div className={`palette find-window${wide ? " roomy" : ""}`} role="dialog" aria-modal="true" aria-label="Find" onMouseDown={(e) => e.stopPropagation()}>
        <div className="find-main">
          <div className="palette-input">
            <Icon name="search" size={16} />
            {doing && (
              <button className="find-only find-doing" onMouseDown={(e) => e.preventDefault()} onClick={back} title="Back (esc)">
                {doing} <Icon name="x" size={10} />
              </button>
            )}
            {!mode && only && scope === "all" && (
              <button className="find-only" onClick={() => setOnly(undefined)} title="Look through everything again">
                {groupTitle[only]} <Icon name="x" size={10} />
              </button>
            )}
            {!mode && scope === "code" && tag && (
              <button className="find-only find-tag" onMouseDown={(e) => e.preventDefault()} onClick={() => setTag(undefined)} title="Search the whole workspace (⌫)">
                repo: {tag} <Icon name="x" size={10} />
              </button>
            )}
            <input
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
                  // ⌘K, with Find open: act on the thing the cursor's on,
                  // a plugin's or ws's own. What can be done to it is the
                  // list.
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
                  // The rest are for finding: ⇥ narrows nothing here.
                  if (e.key === "Tab") e.preventDefault();
                } else if (e.key === "Tab" && !e.shiftKey) {
                  e.preventDefault();
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
            {busy && <Icon name="spinner" size={14} className="find-busy" />}
          </div>
          <div className="find-chips" role="tablist" aria-label="Look through" style={mode ? { display: "none" } : undefined}>
            {chips.map((c) => (
              <button
                key={c.scope}
                role="tab"
                aria-selected={scope === c.scope && !(c.scope === "all" && only)}
                className={scope === c.scope && !(c.scope === "all" && only) ? "on" : ""}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => {
                  setOnly(undefined);
                  setRaw(withScope(raw, c.scope, workspace.root));
                  setCursor(undefined);
                }}
              >
                {c.label}
                {c.scope !== "all" && <kbd>{scopePrefix[c.scope]}</kbd>}
              </button>
            ))}
          </div>
          <div className="find-list" id="palette-list" role="listbox" ref={list} onMouseDown={mode ? (e) => e.preventDefault() : undefined}>
            {mode && (
              <section aria-label={asking ? asking.ask.title : "What can be done to it"}>
                <h3>{asking ? asking.ask.title : `Do to ${acting?.title}`}</h3>
                {rows.map((it) => (
                  <Row key={it.id} it={it} on={it.id === on?.id} section="do" onOver={() => it.id !== on?.id && setCursor(it.id)} onPick={() => choose(it)} />
                ))}
                {asking?.ask.type === "text" && <p className="find-note">Type it, then ⏎ sends it.</p>}
                {!rows.length && asking?.ask.type !== "text" && <p className="palette-none">{typed ? `Nothing here matches “${typed}”.` : "Nothing here."}</p>}
              </section>
            )}
            {!mode &&
              built.sections.map((s) => (
              <section key={s.group} aria-label={s.title}>
                {(built.sections.length > 1 || s.group !== group) && <h3>{s.title}</h3>}
                {s.items.map((it) =>
                  it.code ? (
                    <CodeRow key={it.id} it={it} h={it.code} on={it.id === on?.id} onOver={() => it.id !== on?.id && setCursor(it.id)} onPick={(tab) => go(it, tab)} />
                  ) : (
                    <Row key={it.id} it={it} on={it.id === on?.id} section={s.group} onOver={() => it.id !== on?.id && setCursor(it.id)} onPick={(tab) => go(it, tab)} />
                  ),
                )}
                {s.more > 0 && (
                  <p className="find-more">
                    {s.more} more
                    {!group && s.group !== "best" && <Keys k="⇥" />}
                  </p>
                )}
                {s.note && <p className={`find-note${s.group === "code" && code.end?.capped ? " find-capped" : ""}`}>{s.note}</p>}
              </section>
              ))}
            {nothing && (
              <p className="palette-none">
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
              </p>
            )}
          </div>
          {mode ? (
            <div className="find-foot">
              <span>
                <Keys k="⏎" /> {asking ? (asking.ask.type === "pick" ? "pick it" : "send it") : "do it"}
              </span>
              <span>
                <Keys k="esc" /> back
              </span>
            </div>
          ) : (
            <div className="find-foot">
              <span>
                <Keys k="⏎" /> go
              </span>
              <span>
                <Keys k="mod+⏎" /> new tab
              </span>
              <span>
                <Keys k="⇥" /> {group ? "everything" : "only these"}
              </span>
              {scope === "code" && tag && !q && (
                <span>
                  <Keys k="⌫" /> the whole workspace
                </span>
              )}
              {(onThing || actable) && (
                <span>
                  <Keys k="mod+K" /> act on it
                </span>
              )}
              {on?.ref && (
                <span>
                  <Keys k="mod+C" /> copy its name
                </span>
              )}
              {pinName && (
                <span>
                  <Keys k="mod+." /> {pinnedNow ? "unpin it" : "pin it"}
                </span>
              )}
              {pulls?.at && pullsAge > stale && <span className="find-age">Pull requests as of {clock(pulls.at)}</span>}
            </div>
          )}
        </div>
        {wide && (
          <aside className="find-card ref-card" aria-label="About it">
            {on?.code ? (
              <>
                <span className="hovercard-head">
                  <Icon name="file" size={13} className="dim" />
                  <span className="hovercard-title mono">{on.code.path}</span>
                </span>
                <span className="hovercard-line dim">
                  Line {on.code.line} in {on.code.repo}
                  {on.code.worktree === ".ground" ? ", its default branch" : `, capsule ${on.code.worktree}`}
                </span>
              </>
            ) : on?.pull ? (
              <PullCard it={on} />
            ) : onThing ? (
              // A plugin's thing: its fields, as its plugin has it drawn
              // here. What the search found of it shows until they're in
              // hand, and if they can't be had. A name one of its fields
              // holds is what it names, to go to (which closes Find), and
              // under it is what can be done to it.
              <RefsScope known={known}>
                <div className="find-thing" onClickCapture={(e) => (e.target as HTMLElement).closest(".named-ref, .hovercard-link") && onClose()}>
                  <ThingCard r={onThing} known={known} surface="pane" pointer={pointer} before={!mode && on?.said ? <SaidCard it={on} f={on.said} /> : undefined} />
                </div>
                <div className="find-acts" aria-label="Do to it">
                  {!mode && <PinButton r={onThing} />}
                  {actsOn(namedOf(onThing, known), onThing.id).map((a) => (
                    <Button variant="ghost" size="small" key={`${a.ws ?? "its"}:${a.id}`} type="button" disabled={working} onMouseDown={(e) => e.preventDefault()} onClick={() => doTo(onThing, a)}>
                      <Icon name={actIcon(a.icon)} size={12} /> {a.title}
                    </Button>
                  ))}
                </div>
              </RefsScope>
            ) : on?.said ? (
              <SaidCard it={on} f={on.said} />
            ) : on?.card ? (
              <>
                <RefCardBody r={on.card} known={known} />
                <span className="ref-foot">
                  <PinButton r={on.card} />
                </span>
              </>
            ) : on ? (
              <>
                <span className="hovercard-head">
                  {on.status ? <Status s={on.status} iconOnly size={13} /> : <Icon name={on.icon ?? "chevron-right"} size={13} className="dim" />}
                  <span className="hovercard-title">{on.title}</span>
                </span>
                <span className="hovercard-line dim">{on.kind}</span>
                {on.hint && <span className="hovercard-line dim">{on.hint}</span>}
              </>
            ) : (
              <span className="hovercard-line dim">What the cursor's on shows here.</span>
            )}
          </aside>
        )}
      </div>
    </div>
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

// fileOf is the file a row is, if it's one (a file, a line of one): it
// has its language's icon, as in the trees.
function fileOf(it: Item): string | undefined {
  if (it.icon !== "file") return undefined;
  if (it.code) return it.code.path;
  if (it.card?.kind === "file") return it.card.path;
  if (it.path) return it.title.replace(/:\d+(-\d+)?$/, "");
  return undefined;
}

function Row({ it, on, section, onOver, onPick }: { it: Item; on: boolean; section: GroupKey; onOver: () => void; onPick: (tab: boolean) => void }) {
  const runs = marked(it.title, it.at ?? []);
  const cut = it.path ? Math.max(it.title.lastIndexOf("/"), it.title.lastIndexOf(":")) + 1 : 0;
  let pos = 0;
  // Among other kinds, a row says what it is.
  const says = section === "best" || section === "needs" || section === "linked" || section === "recent";
  return (
    <div id={`pal-${it.id}`} role="option" aria-selected={on} className={`find-row${on ? " on" : ""}`} onMouseMove={onOver} onClick={(e) => onPick(e.metaKey || e.ctrlKey)}>
      {it.status ? <Status s={it.status} iconOnly /> : fileOf(it) ? <FileIcon path={fileOf(it)!} /> : <Icon name={it.icon ?? "chevron-right"} />}
      <span className={`find-title${it.path ? " is-path" : ""}`}>
        <bdi>
          {runs.map((r, i) => {
            const from = pos;
            pos += r.text.length;
            // A run that crosses from a path's folders into its name is split,
            // so the name stands out whole.
            if (it.path && from < cut && pos > cut) {
              return (
                <span key={i}>
                  <span className={r.hit ? "hit" : ""}>{r.text.slice(0, cut - from)}</span>
                  <span className={`name${r.hit ? " hit" : ""}`}>{r.text.slice(cut - from)}</span>
                </span>
              );
            }
            return (
              <span key={i} className={`${r.hit ? "hit" : ""}${it.path && from >= cut ? " name" : ""}`}>
                {r.text}
              </span>
            );
          })}
        </bdi>
      </span>
      {it.hint && <span className="palette-hint">{it.hint}</span>}
      {it.notes?.map((n) => (
        <Chip tone={it.tone && it.tone !== "quiet" ? it.tone : undefined} className="find-chip" key={n}>
          {n}
        </Chip>
      ))}
      {says && <span className="palette-kind">{it.kind}</span>}
    </div>
  );
}

// CodeRow is a line a file says: where (its repo, path and line, and a
// capsule's name), then the line with what matched marked.
function CodeRow({ it, h, on, onOver, onPick }: { it: Item; h: CodeHit; on: boolean; onOver: () => void; onPick: (tab: boolean) => void }) {
  return (
    <div id={`pal-${it.id}`} role="option" aria-selected={on} className={`find-row find-code${on ? " on" : ""}`} onMouseMove={onOver} onClick={(e) => onPick(e.metaKey || e.ctrlKey)}>
      <FileIcon path={h.path} />
      <CodeWhere h={h} repo capsule />
      <CodeText h={h} />
    </div>
  );
}

// SaidCard says of what ws records, found by what it says, where it is
// and what it says there, the words found marked.
function SaidCard({ it, f }: { it: Item; f: Found }) {
  const what = f.what ? capital(f.what) : it.kind;
  const about = [what, f.where, f.on ? `on ${f.on}` : "", f.state && f.place !== "brief" ? f.state : "", f.ended ? "archived" : ""].filter(Boolean);
  return (
    <div className="find-said-card">
      <span className="hovercard-head">
        <Icon name={it.icon ?? "message"} size={13} className="dim" />
        <span className="find-said-title">{it.title}</span>
      </span>
      <span className="dim">{about.join(" · ")}</span>
      {f.snippet && f.snippet.length > 0 && (
        <span className="find-said">
          {f.snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}
        </span>
      )}
    </div>
  );
}

// PullCard says of a pull request what Find was told of it. It doesn't
// ask GitHub: looking down a list shouldn't.
function PullCard({ it }: { it: Item }) {
  const p = it.pull!;
  return (
    <>
      <span className="hovercard-head">
        <Icon name="pr" size={13} className="dim" />
        <span className="hovercard-title">{p.asks ? `${p.repo}#${p.number}` : p.title}</span>
      </span>
      {p.asks ? (
        <span className="hovercard-line dim">It isn't among the open pull requests here. Choosing it asks GitHub.</span>
      ) : (
        <>
          <span className="hovercard-line dim">
            {p.repo}#{p.number}
            {p.author ? `, by ${p.author}` : ""}
            {p.draft ? ", a draft" : ""}
          </span>
          {p.words && (
            <span className="hovercard-line">
              {p.words}
              {p.checks && (
                <Chip tone={p.checks === "success" ? "done" : p.checks === "failure" ? "danger" : "quiet"}>
                  checks {p.checks === "success" ? "pass" : p.checks === "failure" ? "fail" : "running"}
                </Chip>
              )}
            </span>
          )}
          {p.branch && <span className="hovercard-line dim mono">{p.branch}</span>}
          {p.stack && <span className="hovercard-line dim">In a stack: {p.stack}</span>}
        </>
      )}
    </>
  );
}

// useWide is whether there's room for a card beside the list.
function useWide(): boolean {
  const [wide, setWide] = useState(() => window.innerWidth >= 1100);
  useEffect(() => {
    const on = () => setWide(window.innerWidth >= 1100);
    window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, []);
  return wide;
}
