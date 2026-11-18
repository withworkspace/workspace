import { Link } from "react-router";
import { useLoaded } from "../load";
import { artifactStatus, byWords } from "../artifacts/shown";
import {
  ApiError,
  type ArtifactDetail,
  type FileTree,
  type FileView,
  getArtifactDetail,
  type Repo,
  getFile,
  getPluginThing,
  getPR,
  getTree,
  getWorktrees,
  type PluginThing,
  type PR,
  type Workspace,
  type Worktree,
  worktreeAPI,
} from "../api";
import { highlightCode, languageOf, useHighlighter } from "../review/highlight";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { headlineStatus, Status } from "../ui/status";
import { useRefs } from "./known";
import { GROUND, isOutside, type Known, namedOf, type PageRef, refHref } from "./refs";
import { thingIcon } from "./views";
import { FloatingCard } from "../ui/hover";

// RefCard says more about what words name, under them while the
// pointer's there: a file's lines, a pull request's title and where it
// stands, a mission's headline, another page, a conversation, an
// artifact's version and its note, a plugin's thing as its plugin says it
// is. Each has somewhere to go.

// What's been read for cards, kept while the page is open: a file's lines,
// a pull request.
const files = new Map<string, Promise<FileView>>();
const prs = new Map<string, Promise<PR>>();

const fileOf = (repo: string, worktree: string, path: string) => {
  const key = `${repo}\n${worktree}\n${path}`;
  let got = files.get(key);
  if (!got) {
    got = getFile(worktreeAPI(repo, worktree), path);
    files.set(key, got);
    got.catch(() => files.delete(key));
  }
  return got;
};

const prOf = (repo: string, number: number) => {
  const key = `${repo}#${number}`;
  let got = prs.get(key);
  if (!got) {
    got = getPR(repo, number).then((d) => d.pr);
    prs.set(key, got);
    got.catch(() => prs.delete(key));
  }
  return got;
};

// An artifact's versions, read once while the page is open: a card's
// version is written once, and the workspace says when there's another.
const artifacts = new Map<string, Promise<ArtifactDetail>>();
const artifactOf = (id: string, versions: number) => {
  const key = `${id}\n${versions}`;
  let got = artifacts.get(key);
  if (!got) {
    got = getArtifactDetail(id);
    artifacts.set(key, got);
    got.catch(() => artifacts.delete(key));
  }
  return got;
};

// What plugins have said of their things, kept while the page is open:
// null is one its plugin has none of. That, and a plugin that couldn't be
// asked, are asked again the next time.
const things = new Map<string, Promise<PluginThing | null>>();
const thingOf = (plugin: string, what: string, id: string) => {
  const key = `${plugin}\n${what}\n${id}`;
  let got = things.get(key);
  if (!got) {
    got = getPluginThing(plugin, what, id).catch((e: unknown) => {
      things.delete(key);
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    });
    things.set(key, got);
  }
  return got;
};

const trees = new Map<string, Promise<FileTree>>();
const treeOf = (repo: string, worktree: string) => {
  const key = `${repo}\n${worktree}`;
  let got = trees.get(key);
  if (!got) {
    got = getTree(worktreeAPI(repo, worktree));
    trees.set(key, got);
    got.catch(() => trees.delete(key));
  }
  return got;
};

// excerptRoom is how many lines of a file a card shows.
const excerptRoom = 12;

// goneCapsule is whether a ref's capsule has gone since it was named
// (landed, or burned): what there is to name no longer has it.
export const goneCapsule = (r: { repo: string; worktree: string }, known?: Known) =>
  r.worktree !== GROUND && !!known?.capsules && !known.capsules.get(r.repo)?.includes(r.worktree);

// Gone is what a card says of a capsule that's gone, and where its file or
// folder is now: on the default branch, most likely, once it's landed.
function Gone({ r, there }: { r: Extract<PageRef, { kind: "file" | "dir" }>; there: boolean }) {
  return (
    <>
      <span className="hovercard-line dim">
        The capsule {r.worktree} is gone{r.commit ? ` (it was at ${r.commit.slice(0, 7)})` : ""}.{" "}
        {there ? "On the default branch now:" : "It isn't on the default branch either."}
      </span>
      {there && (
        <Link to={refHref({ ...r, worktree: GROUND })} className="hovercard-link">
          On the default branch now <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}

function FileCard({ r: named, known }: { r: Extract<PageRef, { kind: "file" }>; known?: Known }) {
  // A capsule that's gone is read on the default branch, where its file
  // most likely is now.
  const gone = goneCapsule(named, known);
  const r = gone ? { ...named, worktree: GROUND } : named;
  const { value: file, error } = useLoaded(() => fileOf(r.repo, r.worktree, r.path), [r.repo, r.worktree, r.path]);
  useHighlighter();
  // The lines named, with a line either side; or the file's start.
  const from = r.line ? Math.max(1, r.line - 1) : 1;
  const wanted = r.line ? (r.endLine ?? r.line) + 1 : excerptRoom;
  const to = file ? Math.min(file.lines.length, Math.max(wanted, from), from + excerptRoom - 1) : 0;
  const lines = file ? file.lines.slice(from - 1, to) : [];
  const asked = (n: number) => !!r.line && n >= r.line && n <= (r.endLine ?? r.line);
  const where = r.worktree === GROUND ? "default branch" : r.worktree;
  return (
    <>
      <span className="hovercard-head">
        <Icon name="file" size={13} className="dim" />
        <span className="hovercard-title mono">{r.path}</span>
      </span>
      <span className="hovercard-line dim">
        {r.repo}, {where}
        {r.line ? `, line${r.endLine && r.endLine > r.line ? `s ${r.line} to ${r.endLine}` : ` ${r.line}`}` : file ? `, ${file.lines.length} lines` : ""}
      </span>
      {gone && <Gone r={named} there={!!file} />}
      {error && !gone && <span className="hovercard-line dim">It can't be read: {error}</span>}
      {named.commit && !gone && <span className="hovercard-line dim">Named at {named.commit.slice(0, 7)}.</span>}
      {file?.binary && <span className="hovercard-line dim">Not text.</span>}
      {file?.tooLarge && <span className="hovercard-line dim">Too large to show here.</span>}
      {r.line && file && r.line > file.lines.length && <span className="hovercard-line dim">The file has only {file.lines.length} lines now.</span>}
      {lines.length > 0 && (
        <span className="ref-excerpt">
          {lines.map((l, i) => (
            <span key={i} className={`ref-code${asked(from + i) ? " named" : ""}`}>
              <span className="ref-ln">{from + i}</span>
              <span dangerouslySetInnerHTML={{ __html: highlightCode(l, languageOf(r.path)) || " " }} />
            </span>
          ))}
          {file && to < file.lines.length && <span className="ref-code more">…</span>}
        </span>
      )}
      <Link to={refHref(r, known)} className="hovercard-link">
        Open it in Files <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

// DirCard is a folder: what's in it, folders first.
function DirCard({ r: named, known }: { r: Extract<PageRef, { kind: "dir" }>; known?: Known }) {
  const gone = goneCapsule(named, known);
  const r = gone ? { ...named, worktree: GROUND } : named;
  const { value: tree, error } = useLoaded(() => treeOf(r.repo, r.worktree), [r.repo, r.worktree]);
  const inside = new Set<string>();
  for (const f of tree?.files ?? []) {
    if (!f.path.startsWith(r.path + "/")) continue;
    const rest = f.path.slice(r.path.length + 1);
    const slash = rest.indexOf("/");
    inside.add(slash < 0 ? rest : rest.slice(0, slash + 1));
  }
  const names = [...inside].sort((a, b) => Number(b.endsWith("/")) - Number(a.endsWith("/")) || a.localeCompare(b));
  return (
    <>
      <span className="hovercard-head">
        <Icon name="layers" size={13} className="dim" />
        <span className="hovercard-title mono">{r.path}/</span>
      </span>
      <span className="hovercard-line dim">
        {r.repo}, {r.worktree === GROUND ? "default branch" : r.worktree}
        {tree ? `, ${names.length} ${names.length === 1 ? "thing" : "things"} in it` : ""}
      </span>
      {gone && <Gone r={named} there={names.length > 0} />}
      {error && <span className="hovercard-line dim">It can't be read: {error}</span>}
      {names.length > 0 && (
        <span className="ref-excerpt">
          {names.slice(0, excerptRoom).map((n) => (
            <span key={n} className="ref-code">
              <span className="ref-ln" />
              <span>{n}</span>
            </span>
          ))}
          {names.length > excerptRoom && <span className="ref-code more">…</span>}
        </span>
      )}
      <Link to={refHref(r)} className="hovercard-link">
        Open it in Files <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

// RepoCard is a repo: its default branch, and its capsules.
function RepoCard({ r, label }: { r: Extract<PageRef, { kind: "repo" }>; label?: string }) {
  const { value: wts, error } = useLoaded(() => getWorktrees(r.repo), [r.repo]);
  const ground = wts?.find((w: Worktree) => w.ground);
  const capsules = (wts ?? []).filter((w) => !w.ground);
  return (
    <>
      <span className="hovercard-head">
        <Icon name="box" size={13} className="dim" />
        <span className="hovercard-title">{label || r.repo}</span>
        {label && label !== r.repo && <span className="dim mono">{r.repo}</span>}
      </span>
      {ground && (
        <span className="hovercard-line dim">
          Default branch {ground.branch}
          {ground.behind ? `, ${ground.behind} behind` : ""}
        </span>
      )}
      {wts && (
        <span className="hovercard-line dim">
          {capsules.length === 0 ? "No capsules" : `${capsules.length} ${capsules.length === 1 ? "capsule" : "capsules"}: ${capsules.map((c) => c.name).join(", ")}`}
        </span>
      )}
      {error && <span className="hovercard-line dim">It can't be read: {error}</span>}
      <Link to={refHref(r)} className="hovercard-link">
        Open it in Files <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

function PRCard({ r }: { r: Extract<PageRef, { kind: "pr" }> }) {
  const { value: pr, error } = useLoaded(() => prOf(r.repo, r.number), [r.repo, r.number]);
  return (
    <>
      <span className="hovercard-head">
        <Icon name="pr" size={13} className="dim" />
        <span className="hovercard-title">{pr ? pr.title : `${r.repo}#${r.number}`}</span>
      </span>
      {pr && (
        <>
          <span className="hovercard-line dim">
            {r.repo}#{r.number}, by {pr.author}
            {pr.draft ? ", a draft" : ""}
          </span>
          <span className="hovercard-line">
            {pr.words}
            {pr.checks && <span className={`chip ${pr.checks === "success" ? "tone-done" : pr.checks === "failure" ? "tone-danger" : "tone-quiet"}`}>checks {pr.checks === "success" ? "pass" : pr.checks === "failure" ? "fail" : "running"}</span>}
          </span>
          <span className="hovercard-line dim">
            +{pr.additions} −{pr.deletions}, {pr.files} {pr.files === 1 ? "file" : "files"}
          </span>
        </>
      )}
      {error && <span className="hovercard-line dim">GitHub wasn't asked, or has no such pull request here.</span>}
      {!pr && !error && <span className="hovercard-line dim">Asking…</span>}
      <Link to={refHref(r)} className="hovercard-link">
        Review it <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

// ArtifactCard is an artifact, or a version of one: its title, which
// version of how many and where it stands, the version's note, and who
// made it and when.
function ArtifactCard({ r, workspace }: { r: Extract<PageRef, { kind: "artifact" }>; workspace: Workspace }) {
  const brief = workspace.artifacts?.find((a) => a.id === r.id);
  const { value: d, error } = useLoaded(() => (brief ? artifactOf(r.id, brief.versions) : Promise.reject(new Error("not here"))), [r.id, brief?.versions]);
  const n = r.version || brief?.latest?.n || 0;
  const v = d?.all.find((x) => x.n === n) ?? (n === brief?.latest?.n ? brief?.latest : undefined);
  const st = brief && artifactStatus(brief);
  return (
    <>
      <span className="hovercard-head">
        <Icon name="artifact" size={13} className="dim" />
        <span className="hovercard-title">{brief?.title ?? r.id}</span>
      </span>
      {!brief && <span className="hovercard-line dim">This machine has no artifact {r.id}.</span>}
      {brief && (
        <span className="hovercard-line dim">
          {r.version ? `${r.id}.v${r.version}, ` : `${r.id}, `}
          {v ? `v${v.n} of ${brief.versions}` : d ? `it has no v${n}: ${d.all.map((x) => `v${x.n}`).join(", ")}` : `${brief.versions} ${brief.versions === 1 ? "version" : "versions"}`}
          {st && <span className={`chip tone-${st.tone}`}>{st.words}</span>}
        </span>
      )}
      {v?.note && (
        <span className="hovercard-line">
          <Inline text={v.note} lines={4} />
        </span>
      )}
      {v && (
        <span className="hovercard-line dim">
          {byWords(v)}
          {v.from ? `, from ${v.from}` : ""}
          {v.fromTitle ? ` (${v.fromTitle})` : ""}
        </span>
      )}
      {error && brief && <span className="hovercard-line dim">It can't be read: {error}</span>}
      {brief && (
        <Link to={refHref(r)} className="hovercard-link">
          Open the artifact <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}

// ThingCard is a plugin's thing (a Jira issue): what kind of thing it is
// and its name, and what its plugin says of it now: its title, where it
// stands, and whatever else it gives in a line each. It goes where a
// click on its name does.
function ThingCard({ r, known }: { r: Extract<PageRef, { kind: "thing" }>; known?: Known }) {
  const named = namedOf(r, known);
  const { value: thing, error } = useLoaded(() => thingOf(r.plugin, r.what, r.id), [r.plugin, r.what, r.id]);
  const what = r.what.replace(/-/g, " ");
  const href = refHref(r, known);
  return (
    <>
      <span className="hovercard-head">
        <Icon name={thingIcon(named)} size={13} className="dim" />
        <span className="hovercard-title">{thing?.title || r.id}</span>
      </span>
      <span className="hovercard-line dim">
        {named?.title || `${r.plugin} ${what}`} {r.id}
        {thing?.state && <span className="chip tone-quiet">{thing.state}</span>}
      </span>
      {thing?.fields?.map((f, i) => (
        <span key={i} className="hovercard-line dim">
          {f.label}: {f.value}
        </span>
      ))}
      {thing === null && (
        <span className="hovercard-line dim">
          {r.plugin} has no {what} called {r.id}.
        </span>
      )}
      {error && (
        <span className="hovercard-line dim">
          {r.plugin} couldn't be asked about it: {error}
        </span>
      )}
      {thing === undefined && !error && <span className="hovercard-line dim">Asking {r.plugin}…</span>}
      {/* One that isn't there has nowhere to go. */}
      {thing === null ? null : isOutside(href) ? (
        <a href={href} target="_blank" rel="noreferrer" className="hovercard-link">
          Open the {what} <Icon name="external" size={11} />
        </a>
      ) : (
        <Link to={href} className="hovercard-link">
          Open the {what} <Icon name="arrow-right" size={11} />
        </Link>
      )}
    </>
  );
}

function body(r: PageRef, workspace: Workspace, known?: Known, repos?: Map<string, Repo>) {
  switch (r.kind) {
    case "file":
      return <FileCard r={r} known={known} />;
    case "dir":
      return <DirCard r={r} known={known} />;
    case "repo":
      return <RepoCard r={r} label={repos?.get(r.repo)?.label} />;
    case "pr":
      return <PRCard r={r} />;
    case "mission": {
      const m = workspace.missions.find((x) => x.slug === r.slug);
      return (
        <>
          <span className="hovercard-head">
            <Icon name="flag" size={13} className="dim" />
            <span className="hovercard-title">{m?.title ?? r.slug}</span>
          </span>
          {m && (
            <>
              <Status s={headlineStatus(m.headline)} />
              {m.headline.text && (
                <span className="dim">
                  <Inline text={m.headline.text} lines={3} />
                </span>
              )}
            </>
          )}
          <Link to={refHref(r)} className="hovercard-link">
            Open the mission <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "page": {
      const p = workspace.pages?.find((x) => x.id === r.id);
      return (
        <>
          <span className="hovercard-head">
            <Icon name="file" size={13} className="dim" />
            <span className="hovercard-title">{p?.title ?? r.id}</span>
          </span>
          <span className="hovercard-line dim">
            Page {r.id}
            {p?.cells ? `, ${p.cells} ${p.cells === 1 ? "cell" : "cells"}` : ""}
            {p?.findings ? `, ${p.findings} open` : ""}
            {p?.stale ? `, ${p.stale} out of date` : ""}
          </span>
          <Link to={refHref(r)} className="hovercard-link">
            Open the page <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "talk": {
      const t = workspace.conversations.find((x) => x.id === r.id);
      return (
        <>
          <span className="hovercard-head">
            <Icon name="message" size={13} className="dim" />
            <span className="hovercard-title">{t?.title || t?.opening || r.id}</span>
          </span>
          {t?.summary && (
            <span className="dim">
              <Inline text={t.summary} lines={4} />
            </span>
          )}
          <Link to={refHref(r)} className="hovercard-link">
            Open the conversation <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "artifact":
      return <ArtifactCard r={r} workspace={workspace} />;
    case "thing":
      return <ThingCard r={r} known={known} />;
  }
}

// RefCardBody is what a ref's card says, for wherever a card is shown: on
// hover in a page, and beside the list in Find.
export function RefCardBody({ r, workspace, known }: { r: PageRef; workspace: Workspace; known?: Known }) {
  // What there is to name says what a plugin's thing is called, and
  // where it opens: given, where the card's shown outside the pages (Find).
  const refs = useRefs();
  return <>{body(r, workspace, known ?? refs?.known, refs?.repos)}</>;
}

// RefCard is the card for a ref, under the words that name it (el), or
// over them where there's no room below. hint says ⌘-click goes there:
// where words are written, a click is for the cursor.
export function RefCard({ r, el, workspace, onEnter, onLeave, hint = true }: { r: PageRef; el: HTMLElement; workspace: Workspace; onEnter: () => void; onLeave: () => void; hint?: boolean }) {
  // Read here, outside the card: inside it, names are only words.
  const refs = useRefs();
  return (
    <FloatingCard anchor={el} className={`ref-card k-${r.kind}`} onEnter={onEnter} onLeave={onLeave}>
      {body(r, workspace, refs?.known, refs?.repos)}
      {hint && <span className="ref-hint dim">⌘-click its words to go there</span>}
    </FloatingCard>
  );
}
