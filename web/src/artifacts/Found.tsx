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
import { filesWords, widths } from "./shown";

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

// A folder of them, as the list lays them out: the repo, as ws calls it,
// the folders under where promoted artifacts live there ("" for none), and
// the artifacts in it.
interface Folder {
  repo: string;
  label: string;
  group: string;
  found: ArtifactFound[];
}

// inFolders lays them out as they're organised in their repos: by repo,
// in order of what ws calls each, and in a repo by the folders they're
// grouped in, those in none first. Each folder's keep the order they came
// in.
export function inFolders(found: ArtifactFound[]): Folder[] {
  const by = new Map<string, Folder>();
  for (const a of found) {
    const group = a.group ?? "";
    const key = `${a.repo}\n${group}`;
    const folder = by.get(key) ?? { repo: a.repo, label: a.label || a.repo, group, found: [] };
    folder.found.push(a);
    by.set(key, folder);
  }
  return [...by.values()].sort((a, b) => a.label.localeCompare(b.label) || a.repo.localeCompare(b.repo) || a.group.localeCompare(b.group));
}

// FoundArtifacts lists them, under the Artifacts page's own, by repo and
// by the folders they're grouped in there. Nothing's drawn while there are
// none.
export function FoundArtifacts({ workspace }: { workspace: Workspace }) {
  // Read again when the artifacts here change: one promoted here is this
  // machine's, and leaves the list.
  const key = (workspace.artifacts ?? []).map((a) => `${a.id}:${a.status}:${a.home?.where ?? ""}`).join(" ");
  const { value } = useLoaded(() => getArtifactsFound(), [key], { keep: true });
  const found = value?.found ?? [];
  if (found.length === 0) return null;
  const folders = inFolders(found);
  return (
    <section className="artifacts-found" aria-labelledby="artifacts-found-title">
      <h2 id="artifacts-found-title" className="artifacts-found-title">
        In the workspace's repos
      </h2>
      <p className="dim artifacts-found-about">Promoted by someone else, or by you on another machine: each runs from its repo's files.</p>
      {folders.map((f) => (
        <div key={`${f.repo} ${f.group}`} className="artifacts-folder">
          {/* One folder alone, with nothing grouped in it, needs no saying. */}
          {(folders.length > 1 || f.group) && (
            <h3 className="artifacts-folder-name" title={`${f.repo}${f.group ? `: ${f.group}/` : ""}`}>
              <span>{f.label}</span>
              {f.group
                .split("/")
                .filter(Boolean)
                .map((part, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: a folder's parts are in their order, and a name can come twice
                  <span key={i} className="artifacts-folder-part">
                    {part}
                  </span>
                ))}
              <span className="artifacts-folder-count">{f.found.length}</span>
            </h3>
          )}
          <ul className="rows artifacts">
            {f.found.map((a) => (
              <li key={a.name} className="row artifact-list-row">
                <TLink to={foundHref(a.name)} className="row-link">
                  <Icon name="artifact" size={16} className="row-status" />
                  <span className="row-main">
                    <span className="row-line">
                      <span className="title">{a.title}</span>
                      <span className="chip tone-done">Promoted</span>
                    </span>
                    <span className="row-sub artifact-row-meta">
                      <span title={`It lives in ${a.where}`}>
                        <Icon name="branch" size={11} /> {a.label}
                      </span>
                      <span className="mono">{a.where}</span>
                    </span>
                  </span>
                  <span className="age artifact-row-when" title={byWords(a)}>
                    {byWords(a)}
                    {byWords(a) && " · "}
                    <Time iso={a.changed} as="age" />
                  </span>
                </TLink>
              </li>
            ))}
          </ul>
        </div>
      ))}
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
          <span className="chip tone-done">
            <Icon name="check-circle" size={11} />
            Promoted
          </span>
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
              <button key={w.key} aria-pressed={width === w.key} onClick={() => setWidth(w.key)} title={w.title}>
                {w.label}
              </button>
            ))}
          </div>
          <span className="artifact-tools-end">
            <button
              className="ghost small"
              onClick={() => {
                reload();
                setRun((x) => x + 1);
              }}
              disabled={!page}
              title="Run it again, as the repo has it now"
            >
              <Icon name="rotate" size={13} /> Reload <Keys k="r" />
            </button>
            {page && (
              <a className="button-link ghost small" href={d.frame} target="_blank" rel="noopener noreferrer" title="In a tab of its own, still sandboxed">
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
