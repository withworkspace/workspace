import type { Node as PMNode } from "prosemirror-model";
import { Selection, TextSelection } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { FileAs, SaveToFile } from "./FileAs";
import {
  bringIn,
  type CellSource,
  type DismissWhy,
  dismissWhys,
  type DocKind,
  docTo,
  fillCell as startFilling,
  getPage,
  getPageCells,
  getPagePasses,
  filesTo,
  kindHint,
  markPlaced,
  type PagePass,
  type PassFinding,
  runPass,
  settleFinding,
  openCell,
  type PageCell,
  PageChanged,
  type PageDetail,
  type PageFiled,
  removePage,
  savePage,
  type TalkCall,
  type TalkTurn,
  tweakCell,
} from "../api";
import {
  askAbout,
  blocksAt,
  caughtUp,
  type CellHost,
  type CellState,
  cellElement,
  cellMarkdown,
  cellsIn,
  dropCell,
  fillCell,
  keepCell,
  nameCell,
  placeBlocks,
  placeCell,
  plainCell,
  setCells,
  unask,
  wrapCell,
} from "../editor/cells";
import MarkdownEditor, { type Draft } from "../editor/MarkdownEditor";
import { blockOf, type Note, noteElement, onPage, putInstead, setNotes } from "../editor/notes";
import { linkNamed, type RefHost, type RefMark, refreshRefs } from "../editor/refs";
import { Markdown } from "../markdown/LazyMarkdown";
import { Banner } from "../ui/Banner";
import { Calls } from "../ui/Calls";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { useMedia } from "../ui/useMedia";
import { useTitle } from "../ui/useTitle";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { ErrorCard } from "../ui/ErrorCard";
import { Keys } from "../ui/Keys";
import { confirm } from "../ui/Confirm";
import { Time } from "../ui/Time";
import { RefCard } from "../refs/RefCard";
import { useRefs } from "../refs/known";
import { lookOf } from "../refs/views";
import { findRefs, GROUND, isOutside, type Known, type PageRef, parseAddress, refAddress, refHref, refKey, refName, refOf } from "../refs/refs";
import { useLoaded } from "../load";
import { useHoverDelay } from "../ui/useHoverDelay";
import { Chip, chipClass } from "../ui/Chip";
import { type Tone } from "../ui/tone";
import { Count } from "../ui/Count";
import { Button, buttonClass } from "../ui/Button";
import { MoreLink } from "../ui/MoreLink";
import { clip, humanError } from "../ui/text";
import { ToggleButton } from "../ui/ToggleButton";
import { Tab, Tabs } from "../ui/Tabs";
import { Checkbox } from "../ui/Checkbox";
import { Composer } from "../ui/Composer";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { Turn, TurnSaid } from "../ui/Turn";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";

// A page: a document you write, saved as you go, with cells an agent fills
// in. A line that starts with >> asks for something; ⏎ makes it a cell and
// you write on. Its agent's reply is written into the cell when it comes,
// and a cell's panel asks for it to be changed, shows what it read, and
// keeps each version.
//
// The editor is what's true while the page is open: what it holds is saved
// to the page's file, and the server's copy is only read on opening. A
// cell's replies are the server's (they're a conversation), and are written
// into the document here.

// Back is where the page bar's first link goes: the document it's read
// at, or Documents.
export interface Back {
  to: string;
  label: string;
}

export default function PagePage({ id: given, back }: { id?: string; back?: Back }) {
  const params = useParams();
  const id = given ?? params.id ?? "";
  // Reading it again (after it changed somewhere else) starts the editor
  // again from what's there.
  const [reads, setReads] = useState(0);
  const { value: d, error } = useLoaded(() => getPage(id), [id, reads]);
  if (error) {
    return (
      <div className="page narrow">
        <ErrorCard error={humanError(error)} title="Couldn't open this page" retry={() => setReads((n) => n + 1)} />
      </div>
    );
  }
  if (!d) return <Loading what="Opening the page…" />;
  return <Doc key={`${d.id}:${reads}`} d={d} back={back ?? { to: "/documents", label: "Documents" }} reread={() => setReads((n) => n + 1)} />;
}

type Saving = "saved" | "unsaved" | "saving" | "changed" | "failed";

// Draft it asks for a first draft at the page's end: content, so a cell,
// which Keep as text makes yours. What reads the page and says things about
// it (a check, a cold read) is a pass, on a tab of its own, and never in
// the page.
const firstDraft = {
  title: "Ask for a first draft from what's here: its sections are laid out at once, and written one by one. Keep as text makes each yours",
  prompt:
    "a first draft of what this page is working towards, from what it says and what the code shows: the goal, the context, what's in and out, and how we'd know it's done, under a heading each",
};

// A page started as a kind of document is drafted as one: laid out as the
// kind's guide says (its agent is told where the guide is).
const kindDraft = (k: DocKind) => ({
  title: `Ask for a first draft of this ${k.title}, laid out as the workspace's guide to one says${k.guide ? "" : " (it has no guide: as one usually is)"}, from what's here and what the code shows. Keep as text makes each part yours`,
  prompt: `a first draft of this ${k.title}, laid out and written as the guide to one says${k.guide ? ` (${k.guide}: read it first)` : ""}, from what the page says and what the code shows, under a heading each`,
});

// noPasses is a page's passes before they've been read.
const noPasses: PagePass[] = [];
// noKnown is what there is to name outside the web UI's (a test).
const noKnown: Known = { repos: new Map(), missions: [], pages: [], talks: [] };

function Doc({ d, back, reread }: { d: PageDetail; back: Back; reread: () => void }) {
  const workspace = useWorkspace();
  const id = d.id;
  const navigate = useNavigate();
  const [view, setView] = useState<EditorView>();
  const [saving, setSaving] = useState<Saving>("saved");
  const [title, setTitle] = useState(d.title);
  const [open, setOpen] = useState<string>();
  // The kind of document it's written as, and where it was filed, once it
  // was: the page stays, and says so.
  const kind = workspace.kinds?.find((k) => k.name === d.kind);
  const [filed, setFiled] = useState<PageFiled | undefined>(d.filed);
  const [filing, setFiling] = useState(false);
  // The file it's tied to, if it is: saving writes there.
  const [tied, setTied] = useState(d.tied);
  const [savingToFile, setSavingToFile] = useState(false);
  const retie = () => getPage(id).then((x) => setTied(x.tied), () => {});
  // What's changed in the file since, brought into the page (merged with
  // what you've written, and its cells): the page is read again after.
  const [bringing, setBringing] = useState(false);
  const bringItIn = async () => {
    setBringing(true);
    try {
      await save().catch(() => {});
      const r = await bringIn(id);
      toast(r.clashes ? `Brought in. ${r.clashes === 1 ? "One place clashes" : `${r.clashes} places clash`}: marked on the page (<<<<<<<), for you to settle before saving` : "Brought in what changed", { tone: r.clashes ? "info" : "done" });
      reread();
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger" });
    } finally {
      setBringing(false);
    }
  };
  const draftAs = kind ? kindDraft(kind) : firstDraft;
  // The document, as far as what's drawn from it (the map) goes: counted
  // up as it changes.
  const [rev, setRev] = useState(0);
  const [mapOn, setMapOn] = usePref("pageMap");
  const wide = useMedia("(min-width: 1100px)");
  const toggleMap = () => setMapOn((on) => !on);
  useTitle(title);

  // --- Saving: a moment after you stop, one save at a time.
  const draft = useRef<Draft>(undefined);
  const base = useRef(d.version);
  const saved = useRef(d.text);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const inFlight = useRef<Promise<void>>(undefined);
  const viewRef = useRef<EditorView>(undefined);
  viewRef.current = view;
  const stopped = useRef(false); // changed elsewhere: nothing's saved until you choose

  const save = useCallback(
    async (keepalive = false): Promise<void> => {
      clearTimeout(timer.current);
      if (inFlight.current) await inFlight.current.catch(() => {});
      const dr = draft.current;
      if (!dr || stopped.current) return;
      // A cell still waiting for its number is saved once it has it.
      if (viewRef.current && cellsIn(viewRef.current.state.doc).some((c) => !c.node.attrs.id)) return;
      const text = dr.text();
      if (text === saved.current) return setSaving("saved");
      setSaving("saving");
      const run = savePage(id, text, base.current, { keepalive: keepalive && text.length < 60_000 }).then(
        (r) => {
          base.current = r.version;
          saved.current = text;
          setTitle(r.title);
          setSaving(draft.current === dr ? "saved" : "unsaved");
        },
        (e: Error) => {
          if (e instanceof PageChanged) {
            stopped.current = true;
            setSaving("changed");
          } else setSaving("failed");
          throw e;
        },
      );
      inFlight.current = run.finally(() => (inFlight.current = undefined));
      return inFlight.current;
    },
    [id],
  );
  const later = useCallback(() => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => save().catch(() => {}), 700);
  }, [save]);
  const changed = useCallback(
    (dr: Draft) => {
      const first = !draft.current;
      draft.current = dr;
      setRev((n) => n + 1);
      if (first && !dr.dirty) return;
      if (!stopped.current) setSaving("unsaved");
      later();
    },
    [later],
  );
  // What isn't saved yet is, on leaving the page or hiding the tab.
  useEffect(() => {
    const hide = () => document.visibilityState === "hidden" && save(true).catch(() => {});
    document.addEventListener("visibilitychange", hide);
    return () => {
      document.removeEventListener("visibilitychange", hide);
      save().catch(() => {});
    };
  }, [save]);

  // --- Cells
  // Their conversations are read again whenever the workspace says the
  // page has changed: a reply, a run's steps as it goes, or a file one was
  // written from changing.
  const listed = workspace.pages?.find((p) => p.id === id);
  const updated = `${listed?.updated}:${listed?.filling ?? 0}:${listed?.stale ?? 0}:${listed?.findings ?? 0}:${listed?.passing ?? 0}`;

  // --- Passes: each on a tab of its own, beside the page and never in it.
  const loadedPasses = useLoaded(() => getPagePasses(id), [id, updated], { keep: true });
  const passes = loadedPasses.value ?? noPasses;
  const [tab, setTab] = useState("page");
  const putPass = (pass: PagePass) => loadedPasses.set((all) => all?.map((p) => (p.name === pass.name ? pass : p)));
  // Which passes' findings are drawn on the page: yours to choose, pass by
  // pass, and kept in this browser.
  const [onPageOf, setOnPageOf] = usePref("passNotes");
  const shows = (name: string) => onPageOf[name] ?? true;
  const setShows = (name: string, on: boolean) => setOnPageOf((was) => ({ ...was, [name]: on }));
  const noteKey = (pass: string, key: string) => `${pass}:${key}`;
  useEffect(() => {
    if (!view) return;
    const notes: Note[] = [];
    for (const p of passes) {
      if (!(onPageOf[p.name] ?? true)) continue;
      for (const f of p.findings) if (f.state === "open" && f.quote) notes.push({ key: noteKey(p.name, f.key), quote: f.quote, pass: p.name });
    }
    setNotes(view, notes);
  }, [view, passes, onPageOf]);
  // --- Refs: what the page's words name (a file, a pull request, a
  // mission, another page) is drawn as what it is, says more on hover, and
  // is gone to with ⌘-click. Nothing's written into the page for it.
  // What there is to name is the web UI's (refs/known.tsx): this page
  // isn't a ref in itself, and a repo's files are read once it names one.
  const shared = useRefs();
  useEffect(() => {
    // A block a line of its own: a path at a paragraph's end isn't run
    // into the next one's words.
    if (view) shared?.want(view.state.doc.textBetween(0, view.state.doc.content.size, "\n", "\n"));
  }, [view, rev, shared]);
  const known = useMemo<Known>(
    () => (shared ? { ...shared.known, pages: shared.known.pages.filter((p) => p !== id) } : noKnown),
    [shared, id],
  );
  const knownRef = useRef(known);
  knownRef.current = known;
  const workspaceRef = useRef(workspace);
  workspaceRef.current = workspace;
  const reposRef = useRef(shared?.repos ?? new Map());
  reposRef.current = shared?.repos ?? new Map();
  const hovering = useHoverDelay<{ key: string; el: HTMLElement }>(160);
  const hover = hovering.shown;
  // A ref, drawn as its kind's Look says (refs/views.tsx).
  const markOf = (ref: PageRef, from: number, to: number): RefMark => {
    const look = lookOf(ref, { workspace: workspaceRef.current, repos: reposRef.current, things: knownRef.current.things });
    return { from, to, key: refKey(ref), kind: ref.kind, shape: look.shape, accent: look.accent, text: look.text, label: look.after?.label, tone: look.after?.tone };
  };
  const refHost = useMemo<RefHost>(
    () => ({
      find: (text) =>
        findRefs(text, knownRef.current).map((f) => markOf(f.ref, f.from, f.to)),
      // A link to a ref: by its address (as Link what's named writes it).
      at(href, words) {
        const ref = parseAddress(href, knownRef.current);
        return ref && { ...markOf(ref, 0, 0), words: words !== refName(ref) };
      },
      // A moment before it shows, so passing over doesn't flash; and it
      // stays while the pointer's on the card.
      over: (key, el) => hovering.over({ key, el }),
      out: hovering.out,
      follow(key) {
        const r = refOf(key);
        if (!r) return;
        // A plugin's thing with nowhere to open in ws goes to its address
        // outside, in a tab of its own.
        const href = refHref(r, knownRef.current);
        if (isOutside(href)) window.open(href, "_blank", "noopener,noreferrer");
        else navigate(href);
      },
    }),
    [navigate, hovering.over, hovering.out],
  );
  useEffect(() => {
    if (view) refreshRefs(view);
  }, [view, known]);
  // Link what's named: the page's names made links, so it reads and links
  // wherever it goes (GitHub, a pull request's description). Lines are
  // pinned to the commit the repo's files were read at; what only ws has
  // is addressed ws:. One step: undo takes it back.
  const onAnyGitHub = !!workspace.org || Object.keys(workspace.remotes ?? {}).length > 0;
  const linkWhatsNamed = () => {
    if (!view || !onAnyGitHub) return;
    const where = { org: workspace.org, remotes: workspace.remotes, defaultBranch: workspace.defaultBranch ?? "main" };
    const n = linkNamed(view, (text, code) =>
      findRefs(text, knownRef.current)
        .filter((f) => !code || (f.from === 0 && f.to === text.length))
        .map((f) => {
          let ref = f.ref;
          if (ref.kind === "file" && ref.worktree === GROUND && ref.line) {
            const head = shared?.heads.get(ref.repo);
            if (head) ref = { ...ref, commit: head };
          }
          return { from: f.from, to: f.to, href: refAddress(ref, where) };
        }),
    );
    toast(n ? `Linked ${n} ${n === 1 ? "name" : "names"}: undo takes them back` : "Nothing's named here that isn't linked already", { tone: n ? "done" : "info" });
  };
  // What a card's on may have moved, or gone.
  useEffect(() => hovering.clear(), [rev, tab, hovering.clear]);
  const hovered = hover && refOf(hover.key);

  // A note clicked on the page: its finding, in a card under its words.
  const [note, setNote] = useState<string>();
  const noted = useMemo(() => {
    for (const p of passes) for (const f of p.findings) if (noteKey(p.name, f.key) === note) return { pass: p, finding: f };
    return undefined;
  }, [passes, note]);
  // Its cells: as the page came, then as they're read again.
  const loadedCells = useLoaded(() => getPageCells(id), [id, updated], { keep: true });
  const cells = loadedCells.value ?? d.cell;

  const put = (cell: PageCell) =>
    loadedCells.set((cur) => {
      const all = cur ?? d.cell;
      return all.some((c) => c.id === cell.id) ? all.map((c) => (c.id === cell.id ? cell : c)) : [...all, cell];
    });

  // A page reads top to bottom, and so do its cells' agents: a cell asked
  // for under one that's still being filled for the first time waits for
  // it, so "the files above" are there to read. waiting are those asked
  // for and not yet started.
  // How many versions each cell had when last looked at: one more is one
  // just written.
  const written = useRef(new Map(d.cell.map((c) => [c.id, versions(c).length])));
  // The outlines whose blocks this page has put in.
  const placed = useRef(new Set<string>());
  const waiting = useRef(new Set<string>());
  const [waits, setWaits] = useState(0);
  const cellsRef = useRef(cells);
  cellsRef.current = cells;
  const startWaiting = useCallback(async () => {
    const v = viewRef.current;
    if (!v || !waiting.current.size) return;
    const order = cellsIn(v.state.doc);
    // A blank cell above holds one up while it's on its way: not yet
    // numbered, waiting itself, or with its agent at work (or about to be).
    const onItsWay = (name: string) => {
      const c = cellsRef.current.find((x) => x.id === name);
      return !c || waiting.current.has(name) || !!c.answering || c.turns.at(-1)?.by === "you";
    };
    for (const cell of [...waiting.current]) {
      const i = order.findIndex((c) => c.node.attrs.id === cell);
      // (A question above holds nothing, so there's nothing to wait for.)
      const held = i > 0 && order.slice(0, i).some((c) => !c.node.childCount && !c.node.attrs.ask && onItsWay(c.node.attrs.id));
      if (i >= 0 && held) continue;
      waiting.current.delete(cell);
      setWaits((n) => n + 1);
      if (i < 0) continue; // taken out of the page, meanwhile
      await save().catch(() => {});
      await startFilling(id, cell).then(put, (e: Error) => toast(humanError(e.message), { tone: "danger" }));
    }
  }, [id, save]);

  // Each cell is drawn as it stands, and a reply the document hasn't got
  // yet is written into its cell; then those waiting on it can start.
  useEffect(() => {
    if (!view) return;
    const states = new Map(cells.map((c) => [c.id, cellState(c, workspace.root)]));
    for (const cell of waiting.current) states.set(cell, { status: "starting", note: "Waiting for the cell above" });
    setCells(view, states);
    // An outline, just planned: its headings and its blocks, empty, go into
    // the page where it says, so what's coming is there to see and to write
    // around. Once: a block you take out afterwards stays out.
    for (const c of cells) {
      if (!c.blocks?.length || c.placed || placed.current.has(c.id)) continue;
      placed.current.add(c.id);
      placeBlocks(view, c.id, c.blocks);
      markPlaced(id, c.id).catch(() => {});
    }
    // A question about a pass's finding has no place in the page. Asked,
    // in words, for something to be written in, its agent has written it:
    // it takes a place then, under the words its finding is about.
    for (const c of cells) {
      const had = written.current.get(c.id) ?? 0;
      const has = versions(c).length;
      written.current.set(c.id, has);
      if (!c.anchor.finding || has <= had || cellsIn(view.state.doc).some((n) => n.node.attrs.id === c.id)) continue;
      const quote = passes.flatMap((p) => p.findings.map((f) => ({ key: noteKey(p.name, f.key), quote: f.quote }))).find((f) => f.key === c.anchor.finding)?.quote;
      const at = quote ? blockOf(view.state.doc, quote) : undefined;
      placeCell(view, at ? at.to : view.state.doc.content.size, c.id, c.turns[0]?.body ?? "");
    }
    for (const { node } of cellsIn(view.state.doc)) {
      const all = versions(cells.find((c) => c.id === node.attrs.id));
      if (all.length > node.attrs.v) fillCell(view, node.attrs.id, all.length, all[all.length - 1].body);
    }
    startWaiting();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, cells, workspace.root, waits, startWaiting]);

  const host = useMemo<CellHost>(
    () => ({
      // Asked for: it's numbered, the page is saved with it in (its agent
      // reads the page), and then it's filled.
      asked(key, prompt, goal, holds, ask) {
        openCell(id, prompt, { goal, holds, ask })
          .then(async (cell) => {
            const v = viewRef.current;
            if (!v || !nameCell(v, key, cell.id)) return; // undone, meanwhile
            cellsRef.current = [...cellsRef.current, cell];
            waiting.current.add(cell.id);
            put(cell);
            await save();
            await startWaiting();
          })
          .catch((e: Error) => {
            toast(humanError(e.message), { tone: "danger" });
            if (viewRef.current) unask(viewRef.current, key);
          });
      },
      open(cell) {
        if (cell) setOpen((was) => (was === cell ? undefined : cell));
      },
    }),
    [id, save, startWaiting],
  );

  // Asking for words of yours to be changed: the blocks the selection
  // takes in, and where to ask from.
  const [changing, setChanging] = useState<{ from: number; to: number; top: number }>();
  const change = useCallback(() => {
    const v = viewRef.current;
    const host = wrap.current;
    if (!v || !host) return;
    caughtUp(v);
    const range = blocksAt(v.state);
    if (!range) return toast("Put the cursor in the words to change, or select them. A cell is asked from its own panel.", { tone: "info" });
    const end = v.coordsAtPos(range.to - 1);
    setOpen(undefined);
    setChanging({ ...range, top: end.bottom - host.getBoundingClientRect().top + 8 });
  }, []);

  // Words selected in the page (not in a cell): a popover under them asks
  // about them (a question, answered beside the page) or for them to be
  // changed, as selecting text in a document did.
  const [picked, setPicked] = useState<{ top: number; left: number }>();
  const [askingAbout, setAskingAbout] = useState<{ from: number; to: number; words: string; top: number }>();
  useEffect(() => {
    if (!view) return;
    let t: ReturnType<typeof setTimeout>;
    const update = () => {
      clearTimeout(t);
      // (After the editor's taken the selection in.)
      t = setTimeout(() => {
        const v = viewRef.current;
        const box = wrap.current;
        const sel = v?.state.selection;
        if (!v || !box || !sel || !v.hasFocus() || sel.empty || !(sel instanceof TextSelection) || !blocksAt(v.state)) return setPicked(undefined);
        const end = v.coordsAtPos(sel.to);
        const at = box.getBoundingClientRect();
        setPicked({ top: end.bottom - at.top + 6, left: Math.max(0, Math.min(end.left - at.left - 60, at.width - 220)) });
      }, 30);
    };
    document.addEventListener("selectionchange", update);
    return () => {
      clearTimeout(t);
      document.removeEventListener("selectionchange", update);
    };
  }, [view]);
  const askOfSelection = () => {
    const v = viewRef.current;
    const box = wrap.current;
    if (!v || !box) return;
    const range = blocksAt(v.state);
    const { from, to } = v.state.selection;
    if (!range || from === to) return;
    const end = v.coordsAtPos(range.to - 1);
    setPicked(undefined);
    setOpen(undefined);
    setAskingAbout({ ...range, words: v.state.doc.textBetween(from, to, " "), top: end.bottom - box.getBoundingClientRect().top + 8 });
  };

  const opened = cells.find((c) => c.id === open);
  // Asking for a change: if you've edited the cell since it was written,
  // how it now reads goes too, for its agent to start from (and to keep).
  // Asking a question (ask) changes nothing: it's answered in the panel.
  const say = async (cell: PageCell, body: string, ask = false) => {
    await save().catch(() => {});
    const latest = versions(cell).at(-1);
    const now = viewRef.current && cellMarkdown(viewRef.current, cell.id);
    const edited = !ask && now && latest && now !== plainCell(latest.body) ? now : undefined;
    put(await tweakCell(id, cell.id, body, edited, ask));
  };
  const retry = async (cell: PageCell) => {
    await save().catch(() => {});
    put(await startFilling(id, cell.id));
  };

  const remove = async () => {
    if (!(await confirm({ title: `Remove “${title}”?`, body: "It's taken out of your pages. Its file is kept in .ws/pages.", confirm: "Remove", danger: true }))) return;
    stopped.current = true;
    removePage(id).then(
      () => navigate("/documents"),
      (e: Error) => {
        stopped.current = false;
        toast(humanError(e.message), { tone: "danger" });
      },
    );
  };

  // Changed somewhere else: yours, or theirs.
  const keepMine = () =>
    getPage(id).then(
      (now) => {
        base.current = now.version;
        stopped.current = false;
        save().catch(() => {});
      },
      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
    );

  const wrap = useRef<HTMLDivElement>(null);
  const map = useMemo(() => (view ? mapOf(view.state.doc, cells, workspace.root) : undefined), [view, cells, workspace.root, rev]);
  const showMap = mapOn && wide;
  const goTo = (pos: number) => {
    if (!view) return;
    const dom = view.nodeDOM(pos);
    if (dom instanceof HTMLElement) dom.scrollIntoView({ block: "center", behavior: "smooth" });
    view.dispatch(view.state.tr.setSelection(Selection.near(view.state.doc.resolve(Math.min(pos + 1, view.state.doc.content.size)))));
    view.focus();
  };

  const close = useCallback(() => {
    setOpen(undefined);
    viewRef.current?.focus();
  }, []);

  // --- What's done with a pass and what it found
  const failed = (e: Error) => toast(humanError(e.message), { tone: "danger" });
  // Run, the first time or again: over the page as it's saved now.
  const run = async (pass: PagePass) => {
    await save().catch(() => {});
    await runPass(id, pass.name).then(putPass, failed);
  };
  const settle = (pass: PagePass, f: PassFinding, state: PassFinding["state"], why?: DismissWhy) => settleFinding(id, pass.name, f.key, state, why).then(putPass, failed);
  // To a finding's words on the page.
  const toWords = (f: PassFinding) => {
    setTab("page");
    const at = view && f.quote ? blockOf(view.state.doc, f.quote) : undefined;
    // Once the page is showing again.
    if (at) setTimeout(() => goTo(at.from), 0);
  };
  // Its suggestion, taken: the words it quoted read as it proposed, and
  // it's done.
  const take = (pass: PagePass, f: PassFinding) => {
    if (!view || !f.quote || !f.suggest) return;
    if (!putInstead(view, f.quote, f.suggest)) return toast("Its words aren't on the page in one place any more: edit them by hand.", { tone: "info" });
    setNote(undefined);
    settle(pass, f, "done");
  };
  // Found out: a finding asked of an agent. It's answered on the pass's
  // tab, under the finding, and nothing's put in the page for it: it has no
  // place there unless you have its answer written in (putInPage).
  const findOut = async (pass: PagePass, f: PassFinding) => {
    setNote(undefined);
    setTab(pass.name);
    const words = `${f.quote ? `"${f.quote}": ` : ""}${f.text}`;
    await save().catch(() => {});
    await openCell(id, clip(words, 500), { finding: noteKey(pass.name, f.key) })
      .then((cell) => {
        put(cell);
        return startFilling(id, cell.id).then(put);
      })
      .catch(failed);
  };
  // The questions asked of findings, each by its finding: the latest.
  const asked = useMemo(() => {
    const out = new Map<string, PageCell>();
    for (const c of cells) if (c.anchor.finding) out.set(c.anchor.finding, c);
    return out;
  }, [cells]);
  // Its answer, written into the page: the question takes a place there
  // (under the words its finding is about, or at the page's end), and its
  // agent writes what you say, or what it answered, as a cell.
  const putInPage = (f: PassFinding, cell: PageCell, body: string) => {
    if (!view) return;
    setTab("page");
    setTimeout(() => {
      const at = f.quote ? blockOf(view.state.doc, f.quote) : undefined;
      placeCell(view, at ? at.to : view.state.doc.content.size, cell.id, cell.turns[0]?.body ?? "");
      say(cell, body || "Write that up for the page, here.", false).catch(failed);
    }, 0);
  };
  // Asked of the page as a whole (not at a place in it): its agent plans
  // what's to be added and where, the blocks go in empty, and it writes
  // them one by one.
  const [adding, setAdding] = useState(false);
  const plan = (prompt: string) =>
    openCell(id, prompt, { goal: "plan" })
      .then(async (cell) => {
        put(cell);
        await save().catch(() => {});
        put(await startFilling(id, cell.id));
      })
      .catch(failed);
  // The latest such ask that hasn't a plan yet: being planned, or not.
  const planning = [...cells].reverse().find((c) => c.goal === "plan" && !c.blocks?.length);
  const current = passes.find((p) => p.name === tab);

  return (
    <div className={`page page-page${showMap ? " with-map" : " narrow"}`}>
      <header className="page-bar">
        <Link to={back.to} className="back-link">
          <Icon name="arrow-left" size={13} /> {back.label}
        </Link>
        <span className="dim">{id}</span>
        {kind && (
          <Chip tone="quiet" title={kindHint(kind)}>
            {kind.title}
          </Chip>
        )}
        {tied?.in && (
          <Link to={filesTo(tied.in.repo, tied.in.worktree, tied.in.path)} className={chipClass("quiet", "mono page-file")} title={`Tied to ${tied.ref}: saving writes there. Open it in Files`}>
            <Icon name="file" size={11} /> {tied.ref.replace(":", " · ")}
          </Link>
        )}
        <span className="spacer" />
        {planning &&
          (planning.turns.at(-1)?.by === "ws" ? (
            <Button variant="link" className="save-state failed" title={planning.turns.at(-1)?.body} onClick={() => startFilling(id, planning.id).then(put, failed)}>
              <Icon name="alert" size={12} /> Couldn't plan it: try again
            </Button>
          ) : (
            <Chip tone="progress" title={planning.turns[0]?.body}>
              <Spinner size={11} /> Planning{planning.run?.calls.length ? `: ${doing(planning.run.calls.at(-1)!, workspace.root).toLowerCase()}` : "…"}
            </Chip>
          ))}
        <ToggleButton
          variant="ghost" size="small"
          disabled={!view || tab !== "page"}
          pressed={adding}
          title="Ask for something to be added to the page, anywhere in it: a section after Constraints on X, and one after Release on roll out. What's coming is put in at once, and written as you go on"
          onChange={setAdding}
        >
          <Icon name="plus" size={12} /> Add
        </ToggleButton>
        <Button variant="ghost" size="small" disabled={!view || tab !== "page"} title={draftAs.title} onClick={() => plan(draftAs.prompt)}>
          <Icon name="pencil" size={12} /> Draft it
        </Button>
        <Button
          variant="ghost" size="small"
          disabled={!view || tab !== "page"}
          unavailable={onAnyGitHub ? undefined : "The workspace's repos aren't on GitHub to link to (ws.toml's org, or a repo's github)"}
          onClick={linkWhatsNamed}
          title="Make what the page names links (files at their lines, pull requests, repos), so it links on GitHub and wherever else it's read"
        >
          <Icon name="link" size={12} /> Link names
        </Button>
        <Button variant="ghost" size="small" disabled={!view || tab !== "page"} onClick={change} title="Ask for the words the cursor's in (or those selected) to be changed">
          <Icon name="sparkle" size={12} /> Change <Keys k="mod+⏎" />
        </Button>
        {wide && (
          <ToggleButton variant="ghost" size="small" pressed={mapOn} onChange={toggleMap} title="The page's map: its outline, cells, what's open, and the files it rests on">
            <Icon name="panel" size={12} /> Map
          </ToggleButton>
        )}
        {tied && !tied.gone && (
          <Button variant="ghost" size="small" onClick={() => setSavingToFile(true)} unavailable={tied.changed ? "The file's changed since: bring in what changed first" : undefined} title={`Save it to ${tied.ref}, as it reads: its cells as text, its questions left out`}>
            <Icon name="check" size={12} /> Save to repo
          </Button>
        )}
        {!filed && !tied && (
          <Button variant="ghost" size="small" onClick={() => setFiling(true)} title="File it as a document: of one of the workspace's kinds, where the kind says, or in a folder of a repo; landed, or in a capsule lifted for it">
            <Icon name="file" size={12} /> File as…
          </Button>
        )}
        <SaveState s={saving} retry={() => save().catch(() => {})} />
        <Button variant="ghost" size="small" onClick={remove} title="Remove this page">
          Remove
        </Button>
      </header>
      {filing && (
        <FileAs
          id={id}
          kinds={workspace.kinds ?? []}
          kind={d.kind}
          to={d.to}
          spaces={workspace.spaces}
          title={title}
          beforeFile={() => save()}
          onClose={() => setFiling(false)}
          onFiled={(done) => {
            setFiling(false);
            setFiled({ ref: done.ref, kind: done.kind, commit: done.commit, landed: done.how === "land", at: new Date().toISOString(), version: base.current });
            toast(done.message ?? "Filed", { tone: "done" });
            // Tied to it now: saving writes there (and filing may have
            // given the page a heading).
            reread();
          }}
        />
      )}
      {savingToFile && tied && (
        <SaveToFile
          id={id}
          tied={tied}
          beforeSave={() => save()}
          onClose={() => setSavingToFile(false)}
          onSaved={(saved) => {
            setSavingToFile(false);
            toast(saved.message ?? "Saved", { tone: "done" });
            retie();
          }}
          onChanged={(why) => {
            setSavingToFile(false);
            toast(why, { tone: "info" });
            retie();
          }}
        />
      )}
      {tied?.changed && (
        <Banner
          tone="attention"
          role="alert"
          actions={
            <Button size="small" onClick={bringItIn} disabled={bringing}>
              {bringing ? "Bringing it in…" : "Bring in what changed"}
            </Button>
          }
        >
          <span className="mono">{tied.in?.path ?? tied.ref}</span> has changed since this page took it in. Bring in what changed before saving: it's merged with what you've written here.
        </Banner>
      )}
      {tied?.gone && (
        <Banner tone="attention" role="alert">
          The file this page is tied to ({tied.ref}) isn't there any more: {tied.gone}.
        </Banner>
      )}
      {filed && !tied && (
        <Banner
          tone="done"
          icon="check"
          role="status"
          actions={
            <Link to={docTo(filed.ref)} className={buttonClass({ size: "small" })}>
              Open the document
            </Link>
          }
        >
          Filed as <Link to={docTo(filed.ref)} className="mono">{filed.ref.replace(":", " · ")}</Link>
          {filed.landed ? ", landed" : ", in a capsule lifted for it"} <Time iso={filed.at} as="relative" />. It's edited there now: what you change here stays in the page.
        </Banner>
      )}
      {saving === "changed" && (
        <Banner
          tone="attention"
          role="alert"
          actions={
            <>
              <Button size="small" onClick={reread}>
                Read it again
              </Button>
              <Button size="small" onClick={keepMine}>
                Keep what's here
              </Button>
            </>
          }
        >
          This page was changed somewhere else (another tab, perhaps) since you opened it here. Nothing more is saved from here until you choose.
        </Banner>
      )}
      <Tabs className="page-tabs" value={tab} onChange={setTab} aria-label="The page, and passes over it">
        <Tab value="page">Page</Tab>
        {passes.map((p) => {
          const open = p.findings.filter((f) => f.state === "open").length;
          return (
            <Tab key={p.name} value={p.name} title={p.about}>
              {p.title}
              {p.running ? (
                <Spinner size={11} />
              ) : open > 0 ? (
                <Count tone={p.moved ? undefined : "attention"} title={`${open} open${p.moved ? ", from before the page changed" : ""}`}>
                  {open}
                </Count>
              ) : p.failed ? (
                <Icon name="alert" size={11} className="tab-failed" />
              ) : null}
            </Tab>
          );
        })}
      </Tabs>
      {current && (
        <PassTab
          pass={current}
          root={workspace.root}
          doc={view?.state.doc}
          rev={rev}
          shown={shows(current.name)}
          onShown={(on) => setShows(current.name, on)}
          onRun={() => run(current)}
          onSettle={(f, state, why) => settle(current, f, state, why)}
          onWords={toWords}
          onTake={(f) => take(current, f)}
          onFindOut={(f) => findOut(current, f)}
          asked={(f) => asked.get(noteKey(current.name, f.key))}
          onAsk={(cell, body) => say(cell, body, true).catch(failed)}
          onPut={putInPage}
          onCell={(cell) => {
            setTab("page");
            setTimeout(() => setOpen(cell.id), 0);
          }}
        />
      )}
      <div className="page-body" style={tab === "page" ? undefined : { display: "none" }}>
      <div className="page-doc" ref={wrap}>
        <MarkdownEditor
          initial={d.text}
          label="The page"
          placeholder="Write the problem down. Start a line with >> to ask for something there: >> a table of the existing routes"
          autoFocus
          onChange={changed}
          onSubmit={change}
          cells={host}
          refs={refHost}
          onView={setView}
          onNote={(key) => {
            setOpen(undefined);
            setNote((was) => (was === key ? undefined : key));
          }}
        />
        {view && note && noted && (
          <NoteCard
            key={note}
            note={note}
            pass={noted.pass}
            finding={noted.finding}
            view={view}
            wrap={wrap}
            onClose={() => setNote(undefined)}
            onSettle={(state, why) => {
              setNote(undefined);
              settle(noted.pass, noted.finding, state, why);
            }}
            onTake={() => take(noted.pass, noted.finding)}
            onFindOut={() => findOut(noted.pass, noted.finding)}
            onPass={() => {
              setNote(undefined);
              setTab(noted.pass.name);
            }}
          />
        )}
        {view && adding && (
          <ChangeBox
            top={4}
            placeholder="What should be added, and where? A section after Constraints on X, and one after Release on roll out…"
            hint="Its headings and empty blocks go into the page at once, and are written one by one while you go on."
            label="Ask for something to be added to the page"
            onClose={() => {
              setAdding(false);
              view.focus();
            }}
            onAsk={(prompt) => {
              setAdding(false);
              plan(prompt);
              view.focus();
            }}
          />
        )}
        {view && picked && !changing && !askingAbout && !adding && (
          <div className="select-pop" style={{ top: picked.top, left: picked.left }} role="toolbar" aria-label="The words selected" onMouseDown={(e) => e.preventDefault()}>
            <Button variant="ghost" size="small" onClick={askOfSelection} title="Ask about these words: answered beside the page, and nothing's put in it">
              <Icon name="question" size={12} /> Ask
            </Button>
            <Button
              variant="ghost" size="small"
              onClick={() => {
                setPicked(undefined);
                change();
              }}
              title="Ask for these words to be changed: an agent writes them again, and yours are kept to put back"
            >
              <Icon name="sparkle" size={12} /> Change <Keys k="mod+⏎" />
            </Button>
          </div>
        )}
        {view && askingAbout && (
          <ChangeBox
            top={askingAbout.top}
            placeholder="What do you want to know about these words?"
            hint="It's answered beside the page, by an agent that reads the page and the workspace. Nothing's put in the page unless you say."
            label="Ask about the words selected"
            onClose={() => {
              setAskingAbout(undefined);
              view.focus();
            }}
            onAsk={(question) => {
              const asked = askingAbout;
              setAskingAbout(undefined);
              askAbout(view, { from: asked.from, to: asked.to }, asked.words, question, host);
              view.focus();
            }}
          />
        )}
        {view && changing && (
          <ChangeBox
            top={changing.top}
            onClose={() => {
              setChanging(undefined);
              view.focus();
            }}
            onAsk={(prompt) => {
              // What's selected may have moved since: it's asked of what's
              // there now.
              const range = blocksAt(view.state) ?? changing;
              setChanging(undefined);
              wrapCell(view, range, prompt, host);
              view.focus();
            }}
          />
        )}
        {view && opened && (
          <CellPanel
            key={opened.id}
            cell={opened}
            view={view}
            wrap={wrap}
            root={workspace.root}
            onClose={close}
            onSay={(body, ask) => say(opened, body, ask)}
            onRetry={() => retry(opened)}
          />
        )}
      </div>
      {showMap && map && <PageMap map={map} passes={passes} toPass={setTab} goTo={goTo} openCell={(cell) => setOpen(cell)} />}
      {hover && hovered && hover.el.isConnected && (
        <RefCard r={hovered} el={hover.el} onEnter={hovering.stay} onLeave={hovering.out} />
      )}
      </div>
    </div>
  );
}

// ChangeBox asks what should change in words of yours, under them: asked,
// they become a cell, and an agent writes them again.
function ChangeBox({
  top,
  onClose,
  onAsk,
  placeholder = "What should change here? Tighten it, make it a table, add what's missing…",
  hint = "They become a cell: an agent writes them again, and your words are kept to put back.",
  label = "Ask for these words to be changed",
}: {
  top: number;
  onClose: () => void;
  onAsk: (prompt: string) => void;
  placeholder?: string;
  hint?: string;
  label?: string;
}) {
  const box = useRef<HTMLTextAreaElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const [text, setText] = useState("");
  useEffect(() => {
    box.current?.focus();
    const down = (e: MouseEvent) => !panel.current?.contains(e.target as Node) && onClose();
    document.addEventListener("mousedown", down);
    return () => document.removeEventListener("mousedown", down);
  }, [onClose]);
  const ask = () => text.trim() && onAsk(text.trim());
  return (
    <div className="cell-panel change-box" ref={panel} style={{ top, left: 0 }} role="dialog" aria-label={label}>
      <Composer
        layout="beside"
        className="cell-composer"
        field={box}
        value={text}
        onChange={setText}
        onSend={ask}
        tallest={160}
        placeholder={placeholder}
        aria-label={label}
        escape={onClose}
      >
        <Button variant="primary" size="small" disabled={!text.trim()} onClick={ask}>
          Ask <SendKeys />
        </Button>
      </Composer>
      <div className="cell-panel-foot">
        <span className="dim change-hint">{hint}</span>
      </div>
    </div>
  );
}

// --- Passes

const kindTone: Record<string, Tone> = { requirement: "done", vague: "attention", assumption: "waiting", missing: "attention", ambiguity: "attention" };

// FindingBody is what a finding says: its kind, the words it's about, what
// it has to say of them, and the words it proposes in their place.
function FindingBody({ f, lost, onWords }: { f: PassFinding; lost?: boolean; onWords?: () => void }) {
  return (
    <>
      <div className="finding-head">
        <Chip tone={kindTone[f.kind] ?? "quiet"}>{f.kind}</Chip>
        {f.quote &&
          (onWords && !lost ? (
            <button className="finding-quote" onClick={onWords} title="Go to these words on the page">
              {f.quote}
            </button>
          ) : (
            <span className={`finding-quote${lost ? " lost" : ""}`} title={lost ? "These words aren't on the page any more" : undefined}>
              {f.quote}
            </span>
          ))}
      </div>
      <div className="finding-text">
        <Markdown text={f.text} />
      </div>
      {f.suggest && (
        <div className="finding-suggest">
          <span className="dim">Should read</span>
          <Markdown text={f.suggest} />
        </div>
      )}
    </>
  );
}

// PassTab is a pass over the page, on its tab: what it is, when it ran and
// what it said, and what it found, each to deal with. Run again, it reads
// the page as it is now; what you've settled stays settled.
function PassTab({
  pass,
  root,
  doc,
  rev,
  shown,
  onShown,
  onRun,
  onSettle,
  onWords,
  onTake,
  onFindOut,
  asked,
  onAsk,
  onPut,
  onCell,
}: {
  pass: PagePass;
  root: string;
  doc?: PMNode;
  rev: number;
  shown: boolean;
  onShown: (on: boolean) => void;
  onRun: () => Promise<void>;
  onSettle: (f: PassFinding, state: PassFinding["state"], why?: DismissWhy) => void;
  onWords: (f: PassFinding) => void;
  onTake: (f: PassFinding) => void;
  onFindOut: (f: PassFinding) => void;
  // asked is the question asked of a finding, if one has been; onAsk asks
  // more in it, onPut has its answer written into the page, and onCell
  // goes to the cell it's become.
  asked: (f: PassFinding) => PageCell | undefined;
  onAsk: (cell: PageCell, body: string) => Promise<void>;
  onPut: (f: PassFinding, cell: PageCell, body: string) => void;
  onCell: (cell: PageCell) => void;
}) {
  const [starting, setStarting] = useState(false);
  const [settled, setSettled] = useState(false);
  const start = () => {
    setStarting(true);
    onRun().finally(() => setStarting(false));
  };
  const open = pass.findings.filter((f) => f.state === "open");
  const put = pass.findings.filter((f) => f.state !== "open");
  // Whose words have gone from the page since.
  const lost = useMemo(() => new Set(doc ? pass.findings.filter((f) => f.quote && !onPage(doc, f.quote)).map((f) => f.key) : []), [doc, pass.findings, rev]);
  const live = (pass.run?.calls ?? []).map((c, i, list) => (i === list.length - 1 && !c.result ? { ...c, pending: true } : c));
  const busy = starting || !!pass.running;
  return (
    <div className="pass-tab" role="tabpanel" aria-label={pass.title}>
      <header className="pass-head">
        <div className="pass-about">
          <p>{pass.about}</p>
          <p className="dim">
            {pass.skill ? (
              <>
                Its agent follows the workspace's skill <span className="mono">{pass.skill}</span>
                {pass.args ? <>, asked “{pass.args}”</> : null}, and reads what it needs of the workspace.{" "}
              </>
            ) : pass.cold ? (
              "Its agent is given the page and nothing else. "
            ) : (
              "Its agent reads the page and what it needs of the workspace. "
            )}
            What it says is kept here, beside the page: none of it is written into the page.
          </p>
          {pass.unavailable && <p className="pass-unavailable">It can't be run: {pass.unavailable}.</p>}
        </div>
        <Button variant={pass.runs ? undefined : "primary"} disabled={busy || !!pass.unavailable} onClick={start}>
          {busy ? (
            <>
              <Spinner size={12} /> Running…
            </>
          ) : pass.runs ? (
            <>
              <Icon name="rotate" size={12} /> Run again
            </>
          ) : (
            <>
              <Icon name="play" size={12} /> Run
            </>
          )}
        </Button>
      </header>
      {pass.failed && !pass.running && (
        <Banner>The last run couldn't finish. {pass.failed}</Banner>
      )}
      {pass.moved && !pass.running && !!pass.runs && (
        <Banner
          tone="attention"
          actions={
            <Button size="small" disabled={busy} onClick={start}>
              Run again
            </Button>
          }
        >
          The page has changed since this ran: what's below is of the page as it was.
        </Banner>
      )}
      {pass.running && (
        <div className="pass-running">
          <div className="cell-version">
            <Spinner size={12} />
            <span>{pass.runs ? "Reading the page again" : "Reading the page"}</span>
          </div>
          <Calls calls={live} root={root} open live />
        </div>
      )}
      {!!pass.runs && (
        <>
          <div className="pass-meta">
            {pass.at && (
              <span className="dim">
                Ran <Time iso={pass.at} as="clock" />
                {pass.runs > 1 ? `, ${pass.runs} times in all` : ""}
              </span>
            )}
            <span className="spacer" />
            <Checkbox className="pass-shown" checked={shown} onChange={onShown} title="Mark what it found on the words it's about, on the page. They're drawn over the page, not written into it.">
              Show on the page
            </Checkbox>
          </div>
          {pass.summary && (
            <div className="pass-summary prose">
              <Markdown text={pass.summary} />
            </div>
          )}
          {open.length === 0 && put.length === 0 && <p className="dim">It found nothing to raise.</p>}
          <ol className="findings">
            {open.map((f) => (
              <li key={f.key} className="finding">
                <FindingBody f={f} lost={lost.has(f.key)} onWords={() => onWords(f)} />
                <div className="finding-acts">
                  {f.suggest && f.quote && !lost.has(f.key) && (
                    <Button variant="primary" size="small" onClick={() => onTake(f)} title="Put these words on the page in place of those quoted">
                      Use these words
                    </Button>
                  )}
                  <Button size="small" onClick={() => onSettle(f, "done")} title="It's answered, or dealt with">
                    <Icon name="check" size={12} /> Done
                  </Button>
                  {!asked(f) && (
                    <Button variant="ghost" size="small" onClick={() => onFindOut(f)} title="Ask an agent to find this out: it answers here, under the finding, and nothing's put in the page">
                      Find out
                    </Button>
                  )}
                  <DismissAs onDismiss={(why) => onSettle(f, "dismissed", why)} />
                  {lost.has(f.key) && <span className="dim">Its words aren't on the page any more.</span>}
                </div>
                {asked(f) && <FoundOut cell={asked(f)!} root={root} onAsk={(body) => onAsk(asked(f)!, body)} onPut={(body) => onPut(f, asked(f)!, body)} onCell={() => onCell(asked(f)!)} />}
              </li>
            ))}
          </ol>
          {put.length > 0 && (
            <>
              <MoreLink onClick={() => setSettled((v) => !v)} aria-expanded={settled}>
                {settled ? "Hide" : "Show"} {put.length} settled
              </MoreLink>
              {settled && (
                <ol className="findings settled">
                  {put.map((f) => (
                    <li key={f.key} className="finding">
                      <FindingBody f={f} />
                      <div className="finding-acts">
                        <span className="dim">{f.state === "done" ? "Done" : `Dismissed${f.why ? `: ${dismissWhys.find(([w]) => w === f.why)?.[1].toLowerCase()}` : ""}`}</span>
                        <Button variant="ghost" size="small" onClick={() => onSettle(f, "open")}>
                          Open it again
                        </Button>
                      </div>
                    </li>
                  ))}
                </ol>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}

// FoundOut is what an agent found out about a finding, under it on the
// pass's tab: its answers, what you asked after, and a box to ask more, or
// to have it written into the page. Until then nothing of it is in the
// page.
function FoundOut({ cell, root, onAsk, onPut, onCell }: { cell: PageCell; root: string; onAsk: (body: string) => Promise<void>; onPut: (body: string) => void; onCell: () => void }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const written = versions(cell).length > 0;
  const last = cell.turns.at(-1);
  const live = (cell.run?.calls ?? []).map((c, i, list) => (i === list.length - 1 && !c.result ? { ...c, pending: true } : c));
  const ask = () => {
    const body = text.trim();
    if (!body || busy) return;
    setBusy(true);
    onAsk(body)
      .then(() => setText(""))
      .finally(() => setBusy(false));
  };
  return (
    <div className="found-out">
      <ol className="cell-turns">
        {cell.turns.slice(1).map((t) =>
          t.by === "you" ? (
            <li key={t.n} className="cell-turn">
              <Turn by="you" dense landed>
                <TurnSaid written>{t.body}</TurnSaid>
              </Turn>
            </li>
          ) : t.by !== "agent" ? (
            <li key={t.n} className="cell-turn failed">
              <Icon name="alert" size={12} /> {t.body}
            </li>
          ) : t.answer ? (
            <li key={t.n} className="cell-turn answer">
              <Turn by="agent" dense landed>
                <TurnSaid>
                  <Markdown text={t.body} pinned={t.refs} />
                </TurnSaid>
                <Calls calls={t.calls ?? []} root={root} className="cell-calls" />
              </Turn>
            </li>
          ) : (
            <li key={t.n} className="cell-turn version">
              <div className="cell-version">
                <Icon name="sparkle" size={12} />
                <span>Written into the page</span>
                <Button variant="link" onClick={onCell}>
                  Go to it
                </Button>
              </div>
            </li>
          ),
        )}
        {cell.answering && (
          <li className="cell-turn filling">
            <div className="cell-version">
              <Spinner size={12} />
              <span>{askingNow(cell) ? "Finding out" : "Writing it into the page"}</span>
            </div>
            <Calls calls={live} root={root} open live className="cell-calls" />
          </li>
        )}
        {!cell.answering && last?.by === "you" && (
          <li className="cell-turn failed">
            <span className="dim">Its agent hasn't started.</span>
          </li>
        )}
      </ol>
      {!cell.answering && last?.by !== "you" && (
        <Composer
          layout="beside"
          className="cell-composer"
          value={text}
          onChange={setText}
          onSend={ask}
          tallest={120}
          placeholder={written ? "Ask more…" : "Ask more, or say what to put in the page…"}
          aria-label="Ask more about this finding"
          escape="leave"
        >
          {!written && (
            <Button size="small" disabled={busy} onClick={() => onPut(text.trim())} title="Have it written into the page, under the words this is about: what it's answered, or what you say">
              Put it in the page
            </Button>
          )}
          <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={ask} title="Ask: it's answered here, and the page stays as it is">
            Ask <SendKeys />
          </Button>
        </Composer>
      )}
    </div>
  );
}

// NoteCard is a pass's finding, opened from its words on the page: under
// them, with what to do about it.
// DismissAs dismisses a finding, saying why: it's wrong, the page already
// says it, or it's so but not worth acting on. How often each is how well
// a pass does (ws show passes).
function DismissAs({ onDismiss }: { onDismiss: (why: DismissWhy) => void }) {
  const [asking, setAsking] = useState(false);
  if (!asking)
    return (
      <Button variant="ghost" size="small" onClick={() => setAsking(true)} title="It isn't one: don't raise it again. You'll say why">
        Dismiss
      </Button>
    );
  return (
    <span className="dismiss-as" role="group" aria-label="Dismiss it as">
      <span className="dim">Dismiss as</span>
      {dismissWhys.map(([why, label, title]) => (
        <Button variant="ghost" size="small" key={why} title={title} onClick={() => onDismiss(why)}>
          {label}
        </Button>
      ))}
      <Button variant="ghost" size="small" square onClick={() => setAsking(false)} aria-label="Don't dismiss it" title="Don't dismiss it">
        <Icon name="x" size={11} />
      </Button>
    </span>
  );
}

function NoteCard({
  note,
  pass,
  finding,
  view,
  wrap,
  onClose,
  onSettle,
  onTake,
  onFindOut,
  onPass,
}: {
  note: string;
  pass: PagePass;
  finding: PassFinding;
  view: EditorView;
  wrap: React.RefObject<HTMLDivElement | null>;
  onClose: () => void;
  onSettle: (state: PassFinding["state"], why?: DismissWhy) => void;
  onTake: () => void;
  onFindOut: () => void;
  onPass: () => void;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const [at, setAt] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const mark = noteElement(view, note);
    const host = wrap.current;
    if (!mark || !host) return onClose();
    const r = mark.getBoundingClientRect();
    const w = host.getBoundingClientRect();
    setAt({ top: r.bottom - w.top + 6, left: Math.max(0, Math.min(r.left - w.left, w.width - (panel.current?.offsetWidth ?? 440))) });
  }, [view, note, wrap, onClose]);
  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Element;
      if (panel.current?.contains(t) || t.closest?.("[data-note]")) return;
      onClose();
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("mousedown", down);
      document.removeEventListener("keydown", key);
    };
  }, [onClose]);
  return (
    <div className="cell-panel note-panel" ref={panel} style={at ? { top: at.top, left: at.left } : { visibility: "hidden" }} role="dialog" aria-label={`From ${pass.title}`}>
      <div className="note-from">
        <Button variant="link" onClick={onPass} title="Open the pass it's from">
          {pass.title}
        </Button>
        <span className="spacer" />
        <Button variant="ghost" size="small" square onClick={onClose} aria-label="Close" title="Close" keys="esc">
          <Icon name="x" size={13} />
        </Button>
      </div>
      <div className="finding">
        <FindingBody f={finding} />
        <div className="finding-acts">
          {finding.suggest && (
            <Button variant="primary" size="small" onClick={onTake}>
              Use these words
            </Button>
          )}
          <Button size="small" onClick={() => onSettle("done")}>
            <Icon name="check" size={12} /> Done
          </Button>
          <Button variant="ghost" size="small" onClick={onFindOut}>
            Find out
          </Button>
          <DismissAs onDismiss={(why) => onSettle("dismissed", why)} />
        </div>
      </div>
    </div>
  );
}

// --- The map: the page as what it's made of and what it links to

interface MapData {
  outline: { pos: number; level: number; text: string }[];
  cells: { pos: number; name: string; prompt: string; state: CellState }[];
  // open: what's not ticked off: questions a check raised, tasks you wrote.
  open: { pos: number; text: string }[];
  // sources: the files its cells were written from, with how many cells
  // rest on each.
  sources: { src: CellSource; cells: number }[];
}

function mapOf(doc: PMNode, cells: PageCell[], root: string): MapData {
  const map: MapData = { outline: [], cells: [], open: [], sources: [] };
  const at = new Map<string, number>();
  doc.forEach((top, offset) => {
    if (top.type.name === "heading") map.outline.push({ pos: offset, level: top.attrs.level, text: top.textContent });
    if (top.type.name === "cell") {
      const cell = cells.find((c) => c.id === top.attrs.id);
      map.cells.push({ pos: offset, name: top.attrs.id || top.attrs.key, prompt: top.attrs.prompt, state: cell ? cellState(cell, root) : { status: "starting" } });
      for (const src of cell?.sources ?? []) {
        const i = at.get(src.path);
        if (i === undefined) at.set(src.path, map.sources.push({ src, cells: 1 }) - 1);
        else map.sources[i].cells++;
      }
    }
    top.descendants((node, pos) => {
      if (node.type.name === "list_item" && node.attrs.checked === false && node.firstChild?.textContent) {
        map.open.push({ pos: offset + 1 + pos, text: node.firstChild.textContent });
      }
      return true;
    });
  });
  return map;
}

const stateIcon: Record<CellState["status"], IconName> = { starting: "spinner", filling: "spinner", filled: "check", stale: "alert", failed: "alert" };

// PageMap is the page's map, beside it: its outline, its cells and where
// each stands, what's open, and the files it rests on. Each goes to what
// it names.
function PageMap({
  map,
  passes,
  toPass,
  goTo,
  openCell,
}: {
  map: MapData;
  passes: PagePass[];
  toPass: (name: string) => void;
  goTo: (pos: number) => void;
  openCell: (cell: string) => void;
}) {
  const ran = passes.filter((p) => p.runs || p.running);
  const stale = map.cells.filter((c) => c.state.status === "stale").length;
  const filling = map.cells.filter((c) => c.state.status === "filling" || c.state.status === "starting").length;
  const failed = map.cells.filter((c) => c.state.status === "failed").length;
  const nothing = !map.cells.length && !map.open.length && map.outline.length < 2 && !ran.length;
  return (
    <aside className="page-map" aria-label="This page's map">
      {(map.open.length > 0 || stale > 0 || filling > 0 || failed > 0) && (
        <div className="map-sum">
          {map.open.length > 0 && <Chip tone="attention">{map.open.length} open</Chip>}
          {stale > 0 && <Chip tone="attention">{stale} out of date</Chip>}
          {failed > 0 && <Chip tone="danger">{failed} not filled</Chip>}
          {filling > 0 && (
            <Chip tone="progress">
              <Spinner size={11} /> {filling} filling
            </Chip>
          )}
        </div>
      )}
      {nothing && <p className="map-empty">As you write, this is the page in outline: its headings, its cells, what's open, and the files it rests on.</p>}
      {map.outline.length > 1 && (
        <section>
          <h3>Outline</h3>
          <ul>
            {map.outline.map((h) => (
              <li key={h.pos} style={{ paddingLeft: (Math.min(h.level, 4) - 1) * 10 }}>
                <button className="map-item" onClick={() => goTo(h.pos)}>
                  {h.text || "Untitled"}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {map.cells.length > 0 && (
        <section>
          <h3>Cells</h3>
          <ul>
            {map.cells.map((c) => (
              <li key={c.name}>
                <button
                  className={`map-item map-cell s-${c.state.status}`}
                  title={c.state.note ? `${c.prompt}: ${c.state.note}` : c.prompt}
                  onClick={() => {
                    goTo(c.pos);
                    if (c.state.status === "stale" || c.state.status === "failed") openCell(c.name);
                  }}
                >
                  {c.state.status === "filling" || c.state.status === "starting" ? <Spinner size={11} /> : <Icon name={stateIcon[c.state.status]} size={11} />}
                  <span>{clip(c.prompt, 70)}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {ran.length > 0 && (
        <section>
          <h3>Passes</h3>
          <ul>
            {ran.map((p) => {
              const open = p.findings.filter((f) => f.state === "open").length;
              return (
                <li key={p.name}>
                  <button className={`map-item map-cell${p.running ? " s-filling" : p.moved ? " s-stale" : " s-filled"}`} title={p.about} onClick={() => toPass(p.name)}>
                    {p.running ? <Spinner size={11} /> : <Icon name={p.moved ? "alert" : "check"} size={11} />}
                    <span>
                      {p.title}
                      {p.running ? ": running" : `: ${open} open${p.moved ? ", the page has changed since" : ""}`}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
      )}
      {map.open.length > 0 && (
        <section>
          <h3>Open</h3>
          <ul>
            {map.open.map((o) => (
              <li key={o.pos}>
                <button className="map-item map-open" title={o.text} onClick={() => goTo(o.pos)}>
                  {clip(o.text, 90)}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {map.sources.length > 0 && (
        <section>
          <h3>Rests on</h3>
          <ul>
            {map.sources.map(({ src, cells }) => {
              const href = sourceHref(src);
              const name = sourceName(src);
              const inner = (
                <>
                  <Icon name={src.state ? "alert" : "file"} size={11} />
                  <span>{name}</span>
                  {cells > 1 && <span className="dim">×{cells}</span>}
                </>
              );
              const title = `${name}${src.state === "gone" ? ": gone since" : src.state ? ": changed since" : ""}${cells > 1 ? ` (${cells} cells)` : ""}`;
              return (
                <li key={src.path}>
                  {href ? (
                    <Link to={href} className={`map-item map-source${src.state ? " moved" : ""}`} title={title}>
                      {inner}
                    </Link>
                  ) : (
                    <span className={`map-item map-source${src.state ? " moved" : ""}`} title={title}>
                      {inner}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </aside>
  );
}

function SaveState({ s, retry }: { s: Saving; retry: () => void }) {
  if (s === "failed") {
    return (
      <Button variant="link" className="save-state failed" onClick={retry}>
        <Icon name="alert" size={12} /> Not saved: try again
      </Button>
    );
  }
  const words = { saved: "Saved", unsaved: "Saving…", saving: "Saving…", changed: "Not saving" }[s];
  return (
    <span className={`save-state ${s}`} role="status">
      {s === "saved" && <Icon name="check" size={12} />}
      {words}
    </span>
  );
}

// --- A cell, as its conversation says it stands

// versions are what a cell's agent has written, oldest first.
function versions(cell?: PageCell): TalkTurn[] {
  return cell?.turns.filter((t) => t.by === "agent" && !t.answer) ?? [];
}

// asking is whether what a cell's agent has last been asked is a question,
// to be answered beside the page.
const askingNow = (cell: PageCell) => !![...cell.turns].reverse().find((t) => t.by === "you")?.ask;

const shorten = (s: string, root: string) => s.split(root + "/").join("");

// doing is what a step is, in a few words: "Reading api/routes.go".
function doing(c: TalkCall, root: string): string {
  const input = shorten(c.input, root).replace(/^repos\//, "");
  const tail = input.length > 48 ? "…" + input.slice(-47) : input;
  if (/^(read|view|cat)/i.test(c.tool)) return `Reading ${tail}`;
  if (/^(grep|glob|search|find|ls)/i.test(c.tool)) return `Searching for ${tail}`;
  return `Running ${clip(input, 48)}`;
}

function cellState(cell: PageCell, root: string): CellState {
  const last = cell.turns.at(-1);
  // A block of an outline its agent hasn't come to yet.
  if (cell.queued) return { status: "starting", note: "To be written" };
  if (cell.answering) {
    const step = cell.run?.calls.at(-1);
    return { status: "filling", note: step ? doing(step, root) : askingNow(cell) ? "Answering…" : versions(cell).length ? "Writing it again…" : "Starting…" };
  }
  if (last?.by === "ws") return { status: "failed", note: askingNow(cell) ? "Couldn't answer this" : "Couldn't fill this" };
  if (last?.by === "you") return { status: "starting", note: "Not started" };
  // A question, answered: it holds nothing, and says there's an answer.
  if (!versions(cell).length) return { status: "filled", note: "Answered" };
  if (last?.answer) return { status: "filled", note: "With an answer" };
  const moved = changedSources(cell);
  if (moved.length) return { status: "stale", note: `${sourcesChanged(moved)} since` };
  if (cell.moved) return { status: "stale", note: `${besideChanged(cell)} since` };
  // What its agent had to say of it is beside it: its head says there's
  // something.
  return { status: "filled", note: versions(cell).at(-1)?.notes ? "With a note from its agent" : undefined };
}

// besideChanged says what a cell was written beside that's changed.
const besideChanged = (cell: PageCell) => (cell.moved === "page" ? "The page has changed" : "A cell above has changed");

// outOfDate is why a cell's out of date, in a sentence's start, or "".
function outOfDate(cell: PageCell): string {
  const moved = changedSources(cell);
  if (moved.length) return sourcesChanged(moved);
  return cell.moved ? besideChanged(cell) : "";
}

// changedSources are the files a cell was written from that aren't as
// they were.
const changedSources = (cell: PageCell) => (cell.sources ?? []).filter((s) => s.state);

// sourceName is a source as it's said: its repo and file (demo/main.go),
// or in a capsule, with the capsule.
function sourceName(s: CellSource): string {
  if (!s.repo || !s.file) return s.path;
  return s.worktree && s.worktree !== ".ground" ? `${s.repo}/${s.worktree}: ${s.file}` : `${s.repo}/${s.file}`;
}

const baseName = (s: CellSource) => (s.file ?? s.path).split("/").pop() ?? s.path;

// sourcesChanged says what's changed, in a few words: "main.go has
// changed", "3 files have changed".
function sourcesChanged(moved: CellSource[]): string {
  if (moved.length > 1) return `${moved.length} files have changed`;
  return `${baseName(moved[0])} ${moved[0].state === "gone" ? "has gone" : "has changed"}`;
}

// sourceHref is where a source can be read: the Files page, for one in a
// repo.
function sourceHref(s: CellSource): string | undefined {
  if (!s.repo || !s.file || s.state === "gone") return undefined;
  const tree = s.worktree && s.worktree !== ".ground" ? `/${encodeURIComponent(s.worktree)}` : "";
  return `/files/${encodeURIComponent(s.repo)}${tree}?${new URLSearchParams({ path: s.file })}`;
}

// CellPanel is a cell opened: what was asked for, each version and what
// you asked to change between them, what the latest was written from, and
// a box to ask for a change. It sits under the cell's head.
function CellPanel({
  cell,
  view,
  wrap,
  root,
  onClose,
  onSay,
  onRetry,
}: {
  cell: PageCell;
  view: EditorView;
  wrap: React.RefObject<HTMLDivElement | null>;
  root: string;
  onClose: () => void;
  onSay: (body: string, ask?: boolean) => Promise<void>;
  onRetry: () => Promise<void>;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const box = useRef<HTMLTextAreaElement>(null);
  const sendKey = useSendKey();
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [at, setAt] = useState<{ top: number; left: number }>();

  // Under the cell, so what it holds shows while you ask for a change; a
  // cell too long for that has it under its head. Wherever that now is.
  useLayoutEffect(() => {
    const place = () => {
      const el = cellElement(view, cell.id);
      const head = el?.querySelector(".cell-head");
      const host = wrap.current;
      if (!el || !head || !host) return onClose(); // the cell's gone from the document
      const c = el.getBoundingClientRect();
      const r = c.height < window.innerHeight * 0.5 ? c : head.getBoundingClientRect();
      const w = host.getBoundingClientRect();
      const width = panel.current?.offsetWidth ?? 440;
      setAt({ top: r.bottom - w.top + 6, left: Math.max(0, Math.min(c.left - w.left, w.width - width)) });
    };
    place();
    window.addEventListener("resize", place);
    return () => window.removeEventListener("resize", place);
    // Placed again as the cell changes above or around it.
  }, [view, cell, wrap, onClose]);

  // Opened, it's ready for what you want changed.
  const placed = !!at;
  useEffect(() => {
    if (placed) box.current?.focus();
  }, [placed]);

  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Node;
      if (panel.current?.contains(t) || cellElement(view, cell.id)?.querySelector(".cell-head")?.contains(t)) return;
      onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("mousedown", down);
      document.removeEventListener("keydown", key, true);
    };
  }, [view, cell.id, onClose]);

  const run = (what: () => Promise<void>) => {
    setBusy(true);
    what()
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const all = versions(cell);
  const latest = all.at(-1);
  // A question with nothing written yet asks first, and can be told to put
  // it in the page; a cell is changed first, and can just be asked.
  const question = !latest && !!cell.turns[0]?.ask;
  const send = (ask: boolean) => {
    const body = text.trim() || (question && !ask ? "Write that up for the page, here." : "");
    if (!body || busy) return;
    run(() => onSay(body, ask).then(() => setText("")));
  };
  const read = cell.sources ?? [];
  const moved = changedSources(cell);
  const stale = outOfDate(cell);
  // Written again from what's there now: it's asked to, naming what moved.
  const again = () =>
    run(() =>
      onSay(
        moved.length
          ? `Since you wrote this, ${moved.map((m) => `${m.path} ${m.state === "gone" ? "has gone" : "has changed"}`).join(", ")}. Read what's there now, and write the cell again from it.`
          : `${besideChanged(cell)} since you wrote this. Read the page again, and write the cell again from what's there now.`,
      ),
    );
  const last = cell.turns.at(-1);
  const unstarted = !cell.answering && last?.by !== "agent";
  const live = (cell.run?.calls ?? []).map((c, i, list) => (i === list.length - 1 && !c.result ? { ...c, pending: true } : c));
  return (
    <div className="cell-panel" ref={panel} style={at ? { top: at.top, left: at.left } : { visibility: "hidden" }} role="dialog" aria-label="This cell">
      <div className="cell-panel-head">
        <span className="cell-glyph" aria-hidden>
          »
        </span>
        <span className="cell-panel-prompt">{cell.turns[0]?.body}</span>
        <Button variant="ghost" size="small" square onClick={onClose} aria-label="Close" title="Close" keys="esc">
          <Icon name="x" size={13} />
        </Button>
      </div>
      {cell.turns.length > 1 || cell.answering || cell.turns[0]?.edited ? (
        <ol className="cell-turns">
          {cell.turns[0]?.edited && (
            <li className="cell-turn version">
              <div className="cell-version">
                <Icon name="pencil" size={11} />
                <span>Your words, as they were</span>
                <Button variant="link" onClick={() => fillCell(view, cell.id, 0, cell.turns[0].edited!)} title="Put your own words back in the cell">
                  Use this
                </Button>
              </div>
            </li>
          )}
          {cell.turns.slice(1).map((t) => {
            if (t.by === "you") {
              return (
                <li key={t.n} className="cell-turn">
                  <Turn by="you" dense landed>
                    <TurnSaid written>{t.body}</TurnSaid>
                  </Turn>
                  {t.edited && (
                    <div className="cell-version yours">
                      <Icon name="pencil" size={11} />
                      <span>With your edits to it</span>
                      <Button variant="link" onClick={() => fillCell(view, cell.id, 0, t.edited!)} title="Put the cell back as you'd edited it">
                        Use this
                      </Button>
                    </div>
                  )}
                </li>
              );
            }
            if (t.by !== "agent") {
              return (
                <li key={t.n} className="cell-turn failed">
                  <Icon name="alert" size={12} /> {t.body}
                </li>
              );
            }
            if (t.answer) {
              return (
                <li key={t.n} className="cell-turn answer">
                  <Turn by="agent" dense landed>
                    <TurnSaid>
                      <Markdown text={t.body} pinned={t.refs} />
                    </TurnSaid>
                    <Calls calls={t.calls ?? []} root={root} />
                  </Turn>
                </li>
              );
            }
            const k = all.indexOf(t) + 1;
            return (
              <li key={t.n} className="cell-turn version">
                <div className="cell-version">
                  <Icon name="sparkle" size={12} />
                  <span>{all.length > 1 ? `Version ${k}` : "Written"}</span>
                  <Time iso={t.at} as="clock" className="dim" />
                  {t !== latest && (
                    <Button variant="link" onClick={() => fillCell(view, cell.id, k, t.body)} title="Put this version back in the cell">
                      Use this
                    </Button>
                  )}
                </div>
                <Calls calls={t.calls ?? []} root={root} />
                {t.notes && (
                  <div className={`cell-notes${t === latest ? "" : " old"}`}>
                    <span className="dim">Its agent's notes, to you</span>
                    <Markdown text={t.notes} pinned={t.refs} />
                  </div>
                )}
              </li>
            );
          })}
          {cell.answering && (
            <li className="cell-turn filling">
              <div className="cell-version">
                <Spinner size={12} />
                <span>{askingNow(cell) ? "Answering" : all.length ? "Writing it again" : "Filling it in"}</span>
              </div>
              <Calls calls={live} root={root} open live />
            </li>
          )}
        </ol>
      ) : null}
      {read.length > 0 && !cell.answering && (
        <div className="cell-sources">
          <span className="dim">From</span>
          {read.map((src) => {
            const name = sourceName(src);
            const href = sourceHref(src);
            const said = src.state === "gone" ? `${name}: gone since this was written` : src.state ? `${name}: changed since this was written` : name;
            const inner = (
              <>
                <Icon name={src.state ? "alert" : "file"} size={11} />
                {name.length > 40 ? `${name.split("/")[0]}/…/${baseName(src)}` : name}
              </>
            );
            return href ? (
              <Link key={src.path} to={href} className={chipClass(src.state ? "attention" : undefined, "source-chip")} title={`${said}. Open it in Files.`}>
                {inner}
              </Link>
            ) : (
              <Chip tone={src.state ? "attention" : undefined} key={src.path} className="source-chip" title={said}>
                {inner}
              </Chip>
            );
          })}
        </div>
      )}
      {stale && !cell.answering && !unstarted && (
        <div className="cell-stale">
          <span>{stale} since this was written.</span>
          <Button size="small" disabled={busy} onClick={again}>
            Write it again
          </Button>
        </div>
      )}
      {unstarted && (
        <div className="cell-unstarted">
          <span className="dim">{last?.by === "ws" ? (askingNow(cell) ? "It couldn't be answered." : "It couldn't be filled.") : "Its agent hasn't started."}</span>
          <Button size="small" disabled={busy} onClick={() => run(onRetry)}>
            Try again
          </Button>
        </div>
      )}
      <Composer
        layout="beside"
        className="cell-composer"
        field={box}
        value={text}
        onChange={setText}
        // The key that sends is the first of the two; its other way,
        // the other.
        onSend={({ other }) => send(other ? !question : question)}
        tallest={160}
        placeholder={
          question
            ? "Ask more, or say what to put in the page…"
            : latest
              ? "Say what to change: group them by service, drop the last column… Or ask about it."
              : "Add to what you asked for…"
        }
        aria-label={question ? "Ask more, or say what to put in the page" : "Change this cell, or ask about it"}
        escape="leave"
      >
        {question ? (
          <>
            <Button size="small" disabled={busy} onClick={() => send(false)} title="Have it written into the page, here: what it's answered, or what you say" keys={sendKeys(sendKey, true)}>
              Put it in the page
            </Button>
            <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={() => send(true)} title="Ask: it's answered here, and the page stays as it is">
              Ask <SendKeys />
            </Button>
          </>
        ) : (
          <>
            <Button size="small" disabled={!text.trim() || busy} onClick={() => send(true)} title="Ask about it: it's answered here, and the cell stays as it is" keys={sendKeys(sendKey, true)}>
              Just ask
            </Button>
            <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={() => send(false)} title="Have the cell written again, this way">
              Change <SendKeys />
            </Button>
          </>
        )}
      </Composer>
      <div className="cell-panel-foot">
        {!question && (
          <Button
            variant="link"
            title="Leave what it holds in the page as your own words: no longer a cell"
            onClick={() => {
              keepCell(view, cell.id);
              onClose();
            }}
          >
            Keep as text
          </Button>
        )}
        <Button
          variant="link"
          title={question ? "Take the question out of the page" : "Take the cell out of the page, with what it holds"}
          onClick={() => {
            dropCell(view, cell.id);
            onClose();
          }}
        >
          Remove
        </Button>
        <span className="spacer" />
        <span className="dim">{cell.id}</span>
      </div>
    </div>
  );
}
