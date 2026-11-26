import { Selection, TextSelection } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import {
  bringIn,
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
  tweakCell,
  type DismissWhy,
} from "../api";
import {
  askAbout,
  blocksAt,
  caughtUp,
  type CellHost,
  cellMarkdown,
  cellsIn,
  fillCell,
  nameCell,
  placeBlocks,
  placeCell,
  plainCell,
  setCells,
  unask,
  wrapCell,
} from "../editor/cells";
import MarkdownEditor, { type Draft } from "../editor/MarkdownEditor";
import { blockOf, type Note, putInstead, setNotes } from "../editor/notes";
import { linkNamed, type RefHost, type RefMark, refreshRefs } from "../editor/refs";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
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
import { Count } from "../ui/Count";
import { Button, buttonClass } from "../ui/Button";
import { clip, humanError } from "../ui/strings";
import { ToggleButton } from "../ui/ToggleButton";
import { Tab } from "../ui/Tabs";
import { useWorkspace } from "../workspace/store";
import { usePref } from "../prefs";
import { CellPanel } from "./CellPanel";
import { cellState, doing, versions } from "./cell";
import { ChangeBox } from "./ChangeBox";
import { FileAs, SaveToFile } from "./FileAs";
import { FindingNote } from "./FindingNote";
import { mapOf } from "./map";
import { MapView } from "./MapView";
import { PageBody } from "./PageBody";
import { PageDoc } from "./PageDoc";
import { PageFrame } from "./PageFrame";
import { PageHeader } from "./PageHeader";
import { PageRetry } from "./PageRetry";
import { PageSaveState, type Saving } from "./PageSaveState";
import { PageSelectPop } from "./PageSelectPop";
import { PageTabFailed } from "./PageTabFailed";
import { PageTabs } from "./PageTabs";
import { PassTab } from "./PassTab";
import { Text, textClass } from "../ui/Text";
import { Spacer } from "../ui/Spacer";

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
    <PageFrame map={showMap}>
      <PageHeader back={back} id={id}>
        {kind && (
          <Chip tone="quiet" title={kindHint(kind)}>
            {kind.title}
          </Chip>
        )}
        {tied?.in && (
          <Link to={filesTo(tied.in.repo, tied.in.worktree, tied.in.path)} className={chipClass("quiet", textClass("mono"))} title={`Tied to ${tied.ref}: saving writes there. Open it in Files`}>
            <Icon name="file" size={11} /> {tied.ref.replace(":", " · ")}
          </Link>
        )}
        <Spacer />
        {planning &&
          (planning.turns.at(-1)?.by === "ws" ? (
            <PageRetry title={planning.turns.at(-1)?.body} onClick={() => startFilling(id, planning.id).then(put, failed)}>
              Couldn't plan it: try again
            </PageRetry>
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
        <Button variant="ghost" size="small" disabled={!view || tab !== "page"} onClick={change} title="Ask for the words the cursor's in (or those selected) to be changed" keys="mod+⏎">
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
        <PageSaveState state={saving} retry={() => save().catch(() => {})} />
        <Button variant="ghost" size="small" onClick={remove} title="Remove this page">
          Remove
        </Button>
      </PageHeader>
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
          <Text mono>{tied.in?.path ?? tied.ref}</Text> has changed since this page took it in. Bring in what changed before saving: it's merged with what you've written here.
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
          Filed as <Link to={docTo(filed.ref)} className={textClass("mono")}>{filed.ref.replace(":", " · ")}</Link>
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
      <PageTabs value={tab} onChange={setTab}>
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
                <PageTabFailed />
              ) : null}
            </Tab>
          );
        })}
      </PageTabs>
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
      <PageBody map={showMap} hidden={tab !== "page"}>
      <PageDoc ref={wrap}>
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
          <FindingNote
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
          <PageSelectPop top={picked.top} left={picked.left}>
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
              keys="mod+⏎"
            >
              <Icon name="sparkle" size={12} /> Change <Keys k="mod+⏎" />
            </Button>
          </PageSelectPop>
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
      </PageDoc>
      {showMap && map && <MapView map={map} passes={passes} toPass={setTab} goTo={goTo} openCell={(cell) => setOpen(cell)} />}
      {hover && hovered && hover.el.isConnected && (
        <RefCard r={hovered} el={hover.el} onEnter={hovering.stay} onLeave={hovering.out} />
      )}
      </PageBody>
    </PageFrame>
  );
}
