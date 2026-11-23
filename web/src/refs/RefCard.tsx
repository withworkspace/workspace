import { HoverCardHead, HoverCardLine, hoverCardLinkClass, HoverCardTitle } from "../ui/HoverCard";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { useLoaded } from "../load";
import { artifactStatus, byWords } from "../artifacts/shown";
import { type ArtifactDetail, type FileTree, type FileView, getArtifactDetail, type Repo, getFile, getTree, getWorktrees, type Workspace, type Worktree, worktreeAPI } from "../api";
import { highlightCode, languageOf, useHighlighter } from "../review/highlight";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Status } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { useRefs } from "./known";
import { goneCapsule, GROUND, isOutside, type Known, namedOf, type PageRef, refHref } from "./refs";
import { type Surface, thingBlocks } from "./thing";
import { type Pointer, ThingView, useThing } from "./ThingView";
import { thingIcon } from "./views";
import { type Anchor, FloatingCard } from "../ui/FloatingCard";
import { PinButton } from "../shell/focus";
import { prOf } from "./prs";
import { Chip } from "../ui/Chip";
import { useWorkspace } from "../workspace/store";

// RefCard says more about what words name, under them while the
// pointer's there: a file's lines, a pull request's title and where it
// stands, a mission's headline, another page, a conversation, an
// artifact's version and its note, a plugin's thing as its plugin says it
// is. Each has somewhere to go.

// What's been read for cards, kept while the page is open: a file's lines
// (and a pull request, prs.ts).
const files = new Map<string, Promise<FileView>>();

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

const trees = new Map<string, Promise<FileTree>>();
const treeOf = (repo: string, worktree: string) => {
  const key = `${repo}\n${worktree}`;
  let got = trees.get(key);
  if (!got) {
    got = getTree(worktreeAPI(repo, worktree));
    trees.set(key, got);
    // One still being read is asked again next time.
    got.then((t) => t.unread && trees.delete(key)).catch(() => trees.delete(key));
  }
  return got;
};

// excerptRoom is how many lines of a file a card shows.
const excerptRoom = 12;

// Gone is what a card says of a capsule that's gone, and where its file or
// folder is now: on the default branch, most likely, once it's landed.
function Gone({ r, there }: { r: Extract<PageRef, { kind: "file" | "dir" }>; there: boolean }) {
  return (
    <>
      <HoverCardLine dim>
        The capsule {r.worktree} is gone{r.commit ? ` (it was at ${r.commit.slice(0, 7)})` : ""}.{" "}
        {there ? "On the default branch now:" : "It isn't on the default branch either."}
      </HoverCardLine>
      {there && (
        <Link to={refHref({ ...r, worktree: GROUND })} className={hoverCardLinkClass()}>
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
      <HoverCardHead>
        <Icon name="file" size={13} className="dim" />
        <HoverCardTitle className="mono">{r.path}</HoverCardTitle>
      </HoverCardHead>
      <HoverCardLine dim>
        {r.repo}, {where}
        {r.line ? `, line${r.endLine && r.endLine > r.line ? `s ${r.line} to ${r.endLine}` : ` ${r.line}`}` : file ? `, ${file.lines.length} lines` : ""}
      </HoverCardLine>
      {gone && <Gone r={named} there={!!file} />}
      {error && !gone && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      {named.commit && !gone && <HoverCardLine dim>Named at {named.commit.slice(0, 7)}.</HoverCardLine>}
      {file?.binary && <HoverCardLine dim>Not text.</HoverCardLine>}
      {file?.tooLarge && <HoverCardLine dim>Too large to show here.</HoverCardLine>}
      {r.line && file && r.line > file.lines.length && <HoverCardLine dim>The file has only {file.lines.length} lines now.</HoverCardLine>}
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
      <Link to={refHref(r, known)} className={hoverCardLinkClass()}>
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
      <HoverCardHead>
        <Icon name="layers" size={13} className="dim" />
        <HoverCardTitle className="mono">{r.path}/</HoverCardTitle>
      </HoverCardHead>
      <HoverCardLine dim>
        {r.repo}, {r.worktree === GROUND ? "default branch" : r.worktree}
        {tree && !tree.unread ? `, ${names.length} ${names.length === 1 ? "thing" : "things"} in it` : ""}
      </HoverCardLine>
      {tree?.unread && <HoverCardLine dim>Its files are still being read.</HoverCardLine>}
      {gone && !tree?.unread && <Gone r={named} there={names.length > 0} />}
      {error && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
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
      <Link to={refHref(r)} className={hoverCardLinkClass()}>
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
      <HoverCardHead>
        <Icon name="box" size={13} className="dim" />
        <HoverCardTitle>{label || r.repo}</HoverCardTitle>
        {label && label !== r.repo && <span className="dim mono">{r.repo}</span>}
      </HoverCardHead>
      {ground && (
        <HoverCardLine dim>
          Default branch {ground.branch}
          {ground.behind ? `, ${ground.behind} behind` : ""}
        </HoverCardLine>
      )}
      {wts && (
        <HoverCardLine dim>
          {capsules.length === 0 ? "No capsules" : `${capsules.length} ${capsules.length === 1 ? "capsule" : "capsules"}: ${capsules.map((c) => c.name).join(", ")}`}
        </HoverCardLine>
      )}
      {error && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      <Link to={refHref(r)} className={hoverCardLinkClass()}>
        Open it in Files <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

function PRCard({ r }: { r: Extract<PageRef, { kind: "pr" }> }) {
  const { value: pr, error } = useLoaded(() => prOf(r.repo, r.number), [r.repo, r.number]);
  return (
    <>
      <HoverCardHead>
        <Icon name="pr" size={13} className="dim" />
        <HoverCardTitle>{pr ? pr.title : `${r.repo}#${r.number}`}</HoverCardTitle>
      </HoverCardHead>
      {pr && (
        <>
          <HoverCardLine dim>
            {r.repo}#{r.number}, by {pr.author}
            {pr.draft ? ", a draft" : ""}
          </HoverCardLine>
          <HoverCardLine>
            {pr.words}
            {pr.checks && <Chip className="line-chip" tone={pr.checks === "success" ? "done" : pr.checks === "failure" ? "danger" : "quiet"}>checks {pr.checks === "success" ? "pass" : pr.checks === "failure" ? "fail" : "running"}</Chip>}
          </HoverCardLine>
          <HoverCardLine dim>
            +{pr.additions} −{pr.deletions}, {pr.files} {pr.files === 1 ? "file" : "files"}
          </HoverCardLine>
        </>
      )}
      {error && <HoverCardLine dim>GitHub wasn't asked, or has no such pull request here.</HoverCardLine>}
      {!pr && !error && <HoverCardLine dim>Asking…</HoverCardLine>}
      <Link to={refHref(r)} className={hoverCardLinkClass()}>
        Review it <Icon name="arrow-right" size={11} />
      </Link>
    </>
  );
}

// ArtifactCard is an artifact, or a version of one: its title, which
// version of how many and where it stands, the version's note, and who
// made it and when.
function ArtifactCard({ r }: { r: Extract<PageRef, { kind: "artifact" }> }) {
  const artifacts = useWorkspace((w) => w.artifacts);
  const brief = artifacts?.find((a) => a.id === r.id);
  const { value: d, error } = useLoaded(() => (brief ? artifactOf(r.id, brief.versions) : Promise.reject(new Error("not here"))), [r.id, brief?.versions]);
  const n = r.version || brief?.latest?.n || 0;
  const v = d?.all.find((x) => x.n === n) ?? (n === brief?.latest?.n ? brief?.latest : undefined);
  const st = brief && artifactStatus(brief);
  return (
    <>
      <HoverCardHead>
        <Icon name="artifact" size={13} className="dim" />
        <HoverCardTitle>{brief?.title ?? r.id}</HoverCardTitle>
      </HoverCardHead>
      {!brief && <HoverCardLine dim>This machine has no artifact {r.id}.</HoverCardLine>}
      {brief && (
        <HoverCardLine dim>
          {r.version ? `${r.id}.v${r.version}, ` : `${r.id}, `}
          {v ? `v${v.n} of ${brief.versions}` : d ? `it has no v${n}: ${d.all.map((x) => `v${x.n}`).join(", ")}` : `${brief.versions} ${brief.versions === 1 ? "version" : "versions"}`}
          {st && <Chip className="line-chip" tone={st.tone}>{st.words}</Chip>}
        </HoverCardLine>
      )}
      {v?.note && (
        <HoverCardLine>
          <Inline text={v.note} lines={4} />
        </HoverCardLine>
      )}
      {v && (
        <HoverCardLine dim>
          {byWords(v)}
          {v.from ? `, from ${v.from}` : ""}
          {v.fromTitle ? ` (${v.fromTitle})` : ""}
        </HoverCardLine>
      )}
      {error && brief && <HoverCardLine dim>It can't be read: {error}</HoverCardLine>}
      {brief && (
        <Link to={refHref(r)} className={hoverCardLinkClass()}>
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
// ThingCard is a plugin's thing: its fields, as ws has them or as its
// plugin says when asked, drawn as the plugin declared for where it's
// shown (ThingView). Around that is ws's own: that it's being asked after,
// that there's none or the plugin couldn't be asked, and somewhere to go.
// A thing with nothing drawn yet (it's being asked after, there's none, or
// its kind has no fields) is its name, and what one of its kind is called.
//
// before is shown in place of ws's own, while there's nothing of the thing
// to draw: Find's pane has what the search found of it already. pointer
// draws a name one of its fields holds as what it names: Find's pane
// does, and a card under a name leaves it as words.
export function ThingCard({
  r,
  known,
  surface = "card",
  before,
  pointer,
}: {
  r: Extract<PageRef, { kind: "thing" }>;
  known?: Known;
  surface?: Surface;
  before?: ReactNode;
  pointer?: Pointer;
}) {
  const named = namedOf(r, known);
  const { thing, error } = useThing(r);
  const what = r.what.replace(/-/g, " ");
  const href = refHref(r, known);
  const drawn = !!thing && thingBlocks(named, thing, surface).length > 0;
  if (!drawn && before) return <>{before}</>;
  return (
    <>
      {drawn ? (
        <ThingView named={named} record={thing} surface={surface} pointer={pointer} />
      ) : (
        <>
          <HoverCardHead>
            <Icon name={thingIcon(named)} size={13} className="dim" />
            <HoverCardTitle>{r.id}</HoverCardTitle>
          </HoverCardHead>
          <HoverCardLine dim>
            {named?.title || `${r.plugin} ${what}`} {r.id}
          </HoverCardLine>
        </>
      )}
      {thing === null && (
        <HoverCardLine dim>
          {r.plugin} has no {what} called {r.id}.
        </HoverCardLine>
      )}
      {error && (
        <HoverCardLine dim>
          {r.plugin} couldn't be asked about it: {error}
        </HoverCardLine>
      )}
      {thing === undefined && !error && <HoverCardLine dim>Asking {r.plugin}…</HoverCardLine>}
      {/* One that isn't there has nowhere to go. */}
      {thing === null ? null : isOutside(href) ? (
        <a href={href} target="_blank" rel="noreferrer" className={hoverCardLinkClass()}>
          Open the {what} <Icon name="external" size={11} />
        </a>
      ) : (
        <Link to={href} className={hoverCardLinkClass()}>
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
          <HoverCardHead>
            <Icon name="flag" size={13} className="dim" />
            <HoverCardTitle>{m?.title ?? r.slug}</HoverCardTitle>
          </HoverCardHead>
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
          <Link to={refHref(r)} className={hoverCardLinkClass()}>
            Open the mission <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "page": {
      const p = workspace.pages?.find((x) => x.id === r.id);
      return (
        <>
          <HoverCardHead>
            <Icon name="file" size={13} className="dim" />
            <HoverCardTitle>{p?.title ?? r.id}</HoverCardTitle>
          </HoverCardHead>
          <HoverCardLine dim>
            Page {r.id}
            {p?.cells ? `, ${p.cells} ${p.cells === 1 ? "cell" : "cells"}` : ""}
            {p?.findings ? `, ${p.findings} open` : ""}
            {p?.stale ? `, ${p.stale} out of date` : ""}
          </HoverCardLine>
          <Link to={refHref(r)} className={hoverCardLinkClass()}>
            Open the page <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "talk": {
      const t = workspace.conversations.find((x) => x.id === r.id);
      return (
        <>
          <HoverCardHead>
            <Icon name="message" size={13} className="dim" />
            <HoverCardTitle>{t?.title || t?.opening || r.id}</HoverCardTitle>
          </HoverCardHead>
          {t?.summary && (
            <span className="dim">
              <Inline text={t.summary} lines={4} />
            </span>
          )}
          <Link to={refHref(r)} className={hoverCardLinkClass()}>
            Open the conversation <Icon name="arrow-right" size={11} />
          </Link>
        </>
      );
    }
    case "artifact":
      return <ArtifactCard r={r} />;
    case "thing":
      return <ThingCard r={r} known={known} />;
  }
}

// RefCardBody is what a ref's card says, for wherever a card is shown: on
// hover in a page, and beside the list in Find.
export function RefCardBody({ r, known }: { r: PageRef; known?: Known }) {
  const workspace = useWorkspace();
  // What there is to name says what a plugin's thing is called, and
  // where it opens: given, where the card's shown outside the pages (Find).
  const refs = useRefs();
  return <>{body(r, workspace, known ?? refs?.known, refs?.repos)}</>;
}

// RefCard is the card for a ref, under the words that name it (el), or
// over them where there's no room below. The words are an element of the
// page's, or, for a ref in a plugin's page, where the page said they are
// (plugins/ui.ts). hint says ⌘-click goes there:
// where words are written, a click is for the cursor.
export function RefCard({ r, el, onEnter, onLeave, hint = true }: { r: PageRef; el: Anchor; onEnter: () => void; onLeave: () => void; hint?: boolean }) {
  const workspace = useWorkspace();
  // Read here, outside the card: inside it, names are only words.
  const refs = useRefs();
  return (
    <FloatingCard anchor={el} className={`ref-card k-${r.kind}`} onEnter={onEnter} onLeave={onLeave}>
      {body(r, workspace, refs?.known, refs?.repos)}
      <span className="ref-foot">
        {hint && <span className="ref-hint dim">⌘-click its words to go there</span>}
        <PinButton r={r} />
      </span>
    </FloatingCard>
  );
}
