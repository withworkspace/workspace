import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { act, newPage, type Workspace } from "../api";
import { useKeys } from "../keys";
import { archiveMission } from "../mission/archive";
import { RefCardBody } from "../refs/RefCard";
import { marked } from "../ui/fuzzy";
import { Icon } from "../ui/Icon";
import { Status } from "../ui/status";
import { humanError, Keys, toast } from "../ui/kit";
import { setTheme } from "../ui/theme";
import { clock } from "../ui/time";
import { asks, build, type CodeEnd, type CodeHit, codeRematch, codeStream, type Found, findStream, type FoundGroup, type GroupKey, groupTitle, type Item, localItems, parseQuery, rematch, type Scope, scopePrefix, warmFind, withScope } from "./find";
import type { Waiting } from "./notify";
import { CodeText, CodeWhere } from "./CodeHitParts";
import { loadMemory, noteChosen, noteVisit, recentItems, visitOf } from "./recent";

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

// stale is how old the pull requests can be before Find says how old.
const stale = 10 * 60 * 1000;

// codeWait is how long code search waits for typing to pause before it
// asks: ripgrep isn't started for every letter.
const codeWait = 150;

export function Palette({ workspace, waiting, start, pageRepo, onClose, onHelp }: { workspace: Workspace; waiting: Waiting[]; start?: string; pageRepo?: string; onClose: () => void; onHelp: () => void }) {
  const navigate = useNavigate();
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
  const wide = useWide();

  const { scope, q } = parseQuery(raw, workspace.root);
  const group = scope === "all" ? only : scope;
  const local = useMemo(() => localItems(workspace), [workspace]);
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
        memory,
        spaces: workspace.spaces,
        code: scope === "code" ? code : undefined,
      }),
    [q, scope, only, local, found.groups, needs, recent, memory, workspace.spaces, code],
  );
  const rows = built.rows;
  const at = Math.max(
    0,
    rows.findIndex((r) => r.id === cursor),
  );
  const on = rows[at];

  useEffect(() => {
    list.current?.querySelector(".on")?.scrollIntoView({ block: "nearest" });
  }, [on?.id]);

  const move = (d: number) => {
    if (!rows.length) return;
    setCursor(rows[(at + d + rows.length) % rows.length].id);
  };

  const go = (it: Item | undefined, tab: boolean) => {
    if (!it) return;
    const visit = visitOf(it, Date.now());
    if (visit) noteVisit(workspace.root, visit);
    noteChosen(workspace.root, q, it.id);
    if (it.doing) {
      onClose();
      const d = it.doing;
      if ("theme" in d) setTheme(d.theme);
      else if ("help" in d) onHelp();
      else if ("page" in d)
        newPage().then(
          (p) => navigate(`/documents/local/${encodeURIComponent(p.id)}`),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
      else
        (d.act === "archive" ? archiveMission(d.slug) : act(d.slug, d.act, { crew: d.crew })).then(
          (r) => r && toast(r.message, { tone: "done" }),
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        );
      return;
    }
    if (!it.to) return;
    if (tab) {
      window.open(it.to, "_blank", "noopener");
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
    const kind = g === "best" || g === "needs" || g === "recent" ? kindOf(on) : g;
    if (!kind) return;
    if (kind === "pulls" || kind === "files" || kind === "do") setRaw(withScope(raw, kind, workspace.root));
    else setOnly(kind);
    setCursor(undefined);
  };

  const copy = () => {
    if (!on?.ref) return;
    navigator.clipboard?.writeText(on.ref).then(() => toast(`Copied ${on.ref}`));
  };

  useKeys({ Escape: onClose }, { modal: true });

  const pulls = found.groups.find((g) => g.kind === "pulls");
  const pullsAge = pulls?.at ? Date.now() - new Date(pulls.at).getTime() : 0;
  const busy = scope === "code" ? !code.done : !found.done;
  const nothing = !rows.length && (scope === "code" ? code.done : found.done || !asked);
  const placeholder =
    group === "pulls"
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

  return (
    <div className="overlay palette-overlay" onMouseDown={onClose}>
      <div className={`palette find-window${wide ? " roomy" : ""}`} role="dialog" aria-modal="true" aria-label="Find" onMouseDown={(e) => e.stopPropagation()}>
        <div className="find-main">
          <div className="palette-input">
            <Icon name="search" size={16} />
            {only && scope === "all" && (
              <button className="find-only" onClick={() => setOnly(undefined)} title="Look through everything again">
                {groupTitle[only]} <Icon name="x" size={10} />
              </button>
            )}
            {scope === "code" && tag && (
              <button className="find-only find-tag" onMouseDown={(e) => e.preventDefault()} onClick={() => setTag(undefined)} title="Search the whole workspace (⌫)">
                repo: {tag} <Icon name="x" size={10} />
              </button>
            )}
            <input
              autoFocus
              value={raw}
              placeholder={placeholder}
              aria-label="Find"
              role="combobox"
              aria-expanded="true"
              aria-controls="palette-list"
              aria-activedescendant={on ? `pal-${on.id}` : undefined}
              spellCheck={false}
              autoComplete="off"
              onChange={(e) => {
                setRaw(e.target.value);
                setCursor(undefined);
              }}
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
          <div className="find-chips" role="tablist" aria-label="Look through">
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
          <div className="find-list" id="palette-list" role="listbox" ref={list}>
            {built.sections.map((s) => (
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
            {on?.ref && (
              <span>
                <Keys k="mod+C" /> copy its name
              </span>
            )}
            {pulls?.at && pullsAge > stale && <span className="find-age">Pull requests as of {clock(pulls.at)}</span>}
          </div>
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
            ) : on?.said ? (
              <SaidCard it={on} f={on.said} />
            ) : on?.card ? (
              <RefCardBody r={on.card} workspace={workspace} />
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

function Row({ it, on, section, onOver, onPick }: { it: Item; on: boolean; section: GroupKey; onOver: () => void; onPick: (tab: boolean) => void }) {
  const runs = marked(it.title, it.at ?? []);
  const cut = it.path ? Math.max(it.title.lastIndexOf("/"), it.title.lastIndexOf(":")) + 1 : 0;
  let pos = 0;
  // Among other kinds, a row says what it is.
  const says = section === "best" || section === "needs" || section === "recent";
  return (
    <div id={`pal-${it.id}`} role="option" aria-selected={on} className={`find-row${on ? " on" : ""}`} onMouseMove={onOver} onClick={(e) => onPick(e.metaKey || e.ctrlKey)}>
      {it.status ? <Status s={it.status} iconOnly /> : <Icon name={it.icon ?? "chevron-right"} />}
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
        <span key={n} className={`chip${it.tone && it.tone !== "quiet" ? ` tone-${it.tone}` : ""}`}>
          {n}
        </span>
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
      <Icon name="file" />
      <CodeWhere h={h} repo capsule />
      <CodeText h={h} />
    </div>
  );
}

// SaidCard says of what ws records, found by what it says, where it is
// and what it says there, the words found marked.
function SaidCard({ it, f }: { it: Item; f: Found }) {
  const what = f.what ? f.what[0].toUpperCase() + f.what.slice(1) : it.kind;
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
                <span className={`chip ${p.checks === "success" ? "tone-done" : p.checks === "failure" ? "tone-danger" : "tone-quiet"}`}>
                  checks {p.checks === "success" ? "pass" : p.checks === "failure" ? "fail" : "running"}
                </span>
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
