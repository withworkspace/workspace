import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { type ArtifactFound, type ArtifactFoundDetail, getArtifactFound, getArtifactsFound, type Workspace } from "../api";
import { useKeys } from "../keys";
import { useLoaded } from "../load";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/inline";
import { Copyable, Empty, ErrorCard, Keys, Loading, useTitle } from "../ui/kit";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";
import { filesWords, type FoundFolder as Folder, foundTree, widths } from "./shown";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { Row, RowLine, RowMain, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";

// Artifacts that live in the workspace's repos and aren't this machine's:
// someone else promoted them, or you did on another machine. ws finds each
// by the thing.toml in its folder (as it finds a plugin's things in a
// repo), and it's named by that folder, which is its name everywhere. One
// is listed under the artifacts on this machine, and opened on a page of
// its own, where it runs from the repo's files, in the same sandboxed
// frame as a version here does.

// The keys on one's page, as the keys' help lists them.
export const foundArtifactKeys: [string, string][] = [
  ["w", "The next width: fit, 1280, 768, 390"],
  ["r", "Reload: run it again, as its repo has it now"],
  ["O", "Open it alone, in a tab of its own"],
  ["esc", "Back to Artifacts"],
];

// foundHref is a found artifact's page: /artifacts/in/records/artifacts/2026-11-20-horizon-glow/.
export const foundHref = (name: string) => `/artifacts/in/${name.split("/").map(encodeURIComponent).join("/")}`;

// byWords says who promoted the version that's there, where its record
// says, or who changed it last.
function byWords(a: ArtifactFound): string {
  if (a.promoted?.by) return `v${a.promoted.version} promoted by ${a.promoted.by}`;
  return a.changedBy ? `changed by ${a.changedBy}` : "";
}

// Which folders of the tree are folded, kept for the next time the page's
// opened: by repo and path.
const FOLDED = "ws.artifacts.found.folded";
function folded(): Set<string> {
  try {
    const kept: unknown = JSON.parse(localStorage.getItem(FOLDED) ?? "[]");
    return new Set(Array.isArray(kept) ? kept.filter((k): k is string => typeof k === "string") : []);
  } catch {
    return new Set();
  }
}

// FoundRow is one of them, in the list.
function FoundRow({ a }: { a: ArtifactFound }) {
  return (
    <Row className="artifact-list-row">
      <TLink to={foundHref(a.name)} className={rowLinkClass()}>
        <Icon name="artifact" size={16} className={rowStatusClass()}  />
        <RowMain>
          <RowLine>
            <RowTitle>{a.title}</RowTitle>
            <Chip tone="done">Promoted</Chip>
          </RowLine>
          <RowSub className="artifact-row-meta">
            <span title={`It lives in ${a.where}`}>
              <Icon name="branch" size={11} /> {a.label}
            </span>
            <span className="mono">{a.where}</span>
          </RowSub>
        </RowMain>
        <span className="age artifact-row-when" title={byWords(a)}>
          {byWords(a)}
          {byWords(a) && " · "}
          <Time iso={a.changed} as="age" />
        </span>
      </TLink>
    </Row>
  );
}

// FoundFolder is a repo, or a folder of its artifacts, in the list's tree:
// its name and how many are in it, and, unfolded, the artifacts straight
// in it and its folders.
function FoundFolder({ folder }: { folder: Folder }) {
  const id = `${folder.repo}:${folder.path}`;
  const [open, setOpen] = useState(() => !folded().has(id));
  const toggled = (now: boolean) => {
    if (now === open) return;
    setOpen(now);
    try {
      const kept = folded();
      if (now) kept.delete(id);
      else kept.add(id);
      localStorage.setItem(FOLDED, JSON.stringify([...kept]));
    } catch {
      // Nowhere to keep it: it's folded for now.
    }
  };
  return (
    <details className="tree-node" open={open} onToggle={(e) => toggled(e.currentTarget.open)}>
      <summary title={folder.path ? `${folder.repo}: ${folder.path}/` : folder.repo}>
        <span className="tree-name">{folder.name}</span>
        <span className="tree-count">{folder.count}</span>
      </summary>
      <div className="tree-body">
        {folder.found.length > 0 && (
          <Rows className="artifacts">
            {folder.found.map((a) => (
              <FoundRow key={a.name} a={a} />
            ))}
          </Rows>
        )}
        {folder.folders.map((f) => (
          <FoundFolder key={f.path} folder={f} />
        ))}
      </div>
    </details>
  );
}

// FoundArtifacts lists them, under the Artifacts page's own, as a tree:
// each repo, and the folders they're grouped in there. Nothing's drawn
// while there are none.
export function FoundArtifacts({ workspace }: { workspace: Workspace }) {
  // Read again when the artifacts here change: one promoted here is this
  // machine's, and leaves the list.
  const key = (workspace.artifacts ?? []).map((a) => `${a.id}:${a.status}:${a.home?.where ?? ""}`).join(" ");
  const { value } = useLoaded(() => getArtifactsFound(), [key], { keep: true });
  const found = value?.found ?? [];
  if (found.length === 0) return null;
  const tree = foundTree(found);
  return (
    <section className="artifacts-found" aria-labelledby="artifacts-found-title">
      <h2 id="artifacts-found-title" className="artifacts-found-title">
        In the workspace's repos
      </h2>
      <p className="dim artifacts-found-about">Promoted by someone else, or by you on another machine: each runs from its repo's files.</p>
      {/* One repo with nothing grouped in it is a plain list: there's no tree to draw. */}
      {tree.length === 1 && tree[0]?.folders.length === 0 ? (
        <Rows className="artifacts">
          {found.map((a) => (
            <FoundRow key={a.name} a={a} />
          ))}
        </Rows>
      ) : (
        <div className="artifacts-tree">
          {tree.map((repo) => (
            <FoundFolder key={repo.repo} folder={repo} />
          ))}
        </div>
      )}
    </section>
  );
}

// FoundArtifactPage is one of them, running in a frame that fills the
// page: /artifacts/in/<its folder's name>.
export function FoundArtifactPage() {
  const name = `${(useParams()["*"] ?? "").replace(/\/+$/, "")}/`;
  const navigate = useNavigate();
  const { value: d, error, reload } = useLoaded<ArtifactFoundDetail>(() => getArtifactFound(name), [name], { keep: true });
  useTitle(d ? d.title : name);
  const [width, setWidth] = useState(widths[0].key);
  const [run, setRun] = useState(0); // reload: a new frame
  useKeys({
    w: () => setWidth((w) => widths[(widths.findIndex((x) => x.key === w) + 1) % widths.length].key),
    r: () => {
      reload();
      setRun((x) => x + 1);
    },
    O: () => d && window.open(d.frame, "_blank", "noopener,noreferrer"),
    Escape: () => navigate("/artifacts"),
  });

  if (error && !d) {
    return (
      <div className="page narrow">
        <ErrorCard error={error} retry={reload} title={`Couldn't open ${name}`} />
        <p>
          <Link to="/artifacts">Back to Artifacts</Link>
        </p>
      </div>
    );
  }
  if (!d) return <Loading what={`Opening ${name}…`} />;
  const px = widths.find((w) => w.key === width)?.px ?? 0;
  const page = d.files.some((f) => f.path === "index.html");
  const there = d.promoted ? d.all.find((v) => v.n === d.promoted?.version) : d.all[d.all.length - 1];

  return (
    <div className="artifact-view">
      <header className="artifact-head">
        <div className="artifact-title-line">
          <Link to="/artifacts" className="ghost icon-button small" aria-label="Back to Artifacts" title="Back to Artifacts">
            <Icon name="arrow-left" size={14} />
          </Link>
          <h1>{d.title}</h1>
          <Chip tone="done">
            <Icon name="check-circle" size={11} />
            Promoted
          </Chip>
          <Copyable text={d.where} className="ref" title="Copy where it lives" />
        </div>
        <div className="artifact-note dim">
          <Icon name="branch" size={12} /> It lives in {d.label}, at <span className="mono">{d.where}</span>
          {d.promoted ? (
            <>
              : v{d.promoted.version}, promoted <Time iso={d.promoted.at} as="relative" /> by {d.promoted.by}
            </>
          ) : d.changedBy ? (
            <>
              , last changed <Time iso={d.changed} as="relative" /> by {d.changedBy}
            </>
          ) : null}
          . {filesWords(d.files.length, d.size)}. It isn't on this machine: it runs from the repo's files.
        </div>
        {there?.note && (
          <div className="artifact-note">
            <Inline text={there.note} />
          </div>
        )}
        <div className="artifact-tools">
          <div className="segmented" role="group" aria-label="Width">
            {widths.map((w) => (
              <Button key={w.key} aria-pressed={width === w.key} onClick={() => setWidth(w.key)} title={w.title}>
                {w.label}
              </Button>
            ))}
          </div>
          <span className="artifact-tools-end">
            <Button
              variant="ghost" size="small"
              onClick={() => {
                reload();
                setRun((x) => x + 1);
              }}
              disabled={!page}
              title="Run it again, as the repo has it now"
            >
              <Icon name="rotate" size={13} /> Reload <Keys k="r" />
            </Button>
            {page && (
              <a className={buttonClass({ variant: "ghost" }, "small")} href={d.frame} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed">
                <Icon name="external" size={13} /> Open alone
              </a>
            )}
          </span>
        </div>
      </header>
      <div className="artifact-body">
        <div className="artifact-stage">
          {page ? (
            <div className={`artifact-frame-box${px ? " sized" : ""}`}>
              <iframe key={`${d.version}#${run}`} className="artifact-frame" src={d.frame} sandbox="allow-scripts" referrerPolicy="no-referrer" title={d.title} style={px ? { width: px } : undefined} />
            </div>
          ) : (
            <Empty icon="alert" title="There's no file to run">
              {d.where} has no index.html.
            </Empty>
          )}
        </div>
      </div>
    </div>
  );
}
