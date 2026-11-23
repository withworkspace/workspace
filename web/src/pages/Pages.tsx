import { lazy, type ReactNode, Suspense, useState } from "react";
import { useNavigate } from "react-router";
import { type DocKind, kindHint, newPage, pageAddress, type PageInfo, type Workspace } from "../api";
import type { Back } from "./Page";
import { Icon } from "../ui/Icon";
import { humanError, Keys, Loading, toast } from "../ui/kit";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Time } from "../ui/time";
import { TLink } from "../ui/TLink";
import { Chip } from "../ui/Chip";
import { Button } from "../ui/Button";
import { Row, RowLine, RowMain, RowTitle, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Input } from "../ui/Input";

// Pages: documents you write here, before or beside any mission. A line
// that starts with >> asks for something (a table of the routes, what
// calls this), and an agent fills it in while you write on. They're
// listed in Documents (documents/), where each will live.

export const pageKeys: [string, string][] = [
  [">> then ⏎", "Ask for something there: an agent fills it in"],
  [">? then ⏎", "Ask a question there: it's answered beside the page, and put in it only if you say"],
  ["mod+⏎", "In a cell: ask for a change, or see what it read. Elsewhere: ask for the words there to be changed"],
  ["esc", "Close a cell's panel"],
];

// The page itself (the editor, and so ProseMirror) loads with the first
// page you open.
const Inner = lazy(() => import("./Page"));

export function PagePage({ workspace, id, back }: { workspace: Workspace; id?: string; back?: Back }) {
  return (
    <Suspense fallback={<Loading what="Opening the page…" />}>
      <Inner workspace={workspace} id={id} back={back} />
    </Suspense>
  );
}

// useStartPage starts a page (as one of the workspace's kinds, when one's
// given) and opens it.
export function useStartPage() {
  const navigate = useNavigate();
  const [starting, setStarting] = useState(false);
  const start = (title = "", kind = "", to = "") => {
    if (starting) return;
    setStarting(true);
    newPage(title, kind, to)
      .then((d) => navigate(`/documents/local/${encodeURIComponent(d.id)}`))
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setStarting(false));
  };
  return { start, starting };
}

// PageRow is a page in a list: its title, its kind, the file it's tied to,
// its cells, and when it last changed. note says more of it, after its
// chips (where it's filed, say).
export function PageRow({ page: p, kinds, selected, onHover, note }: { page: PageInfo; kinds: DocKind[]; selected: boolean; onHover: () => void; note?: ReactNode }) {
  return (
    <Row selected={selected} id={`page-${p.id}`} role="option" aria-selected={selected} onMouseMove={onHover}>
      <TLink to={pageAddress(p)} className={rowLinkClass("page-link")}>
        <Icon name="file" size={16} className={rowStatusClass("page-status")} />
        <RowMain>
          <RowLine>
            <RowTitle>{p.title}</RowTitle>
            {p.kind && <KindChip kind={kinds.find((k) => k.name === p.kind)} name={p.kind} />}
            {p.source && (
              <Chip tone="quiet" className="mono" title={`Tied to ${p.source.ref}: saving it writes there`}>
                <Icon name="file" size={11} /> {p.source.ref.replace(/^[^:]*:/, "").split("/").pop()}
              </Chip>
            )}
            {!!p.filling && (
              <Chip tone="progress">
                <Icon name="spinner" size={11} className="spin" /> Filling {p.filling === 1 ? "a cell" : `${p.filling} cells`}
              </Chip>
            )}
            {!!p.cells && !p.filling && <Chip tone="quiet">{p.cells === 1 ? "1 cell" : `${p.cells} cells`}</Chip>}
            {!!p.stale && !p.filling && (
              <Chip tone="attention" title="Written from files that have changed since">
                {p.stale === 1 ? "1 out of date" : `${p.stale} out of date`}
              </Chip>
            )}
            {note}
          </RowLine>
        </RowMain>
        <span className="dim page-ref">{p.id}</span>
        <Time iso={p.updated} as="age" className="age" />
      </TLink>
    </Row>
  );
}

function KindChip({ kind, name }: { kind?: DocKind; name: string }) {
  return (
    <Chip tone="quiet" title={kind ? kindHint(kind) : `A kind the workspace no longer has (${name})`}>
      {kind?.title ?? name}
    </Chip>
  );
}

// slug is a title made into a file's name, as ws makes it
// (workspace.Slug): lowercase words and digits joined by hyphens, cut at a
// word to 60.
const slug = (title: string) => {
  let s = title.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).join("-");
  if (s.length > 60) {
    const cut = s.slice(0, 60);
    const at = cut.lastIndexOf("-");
    s = (s[60] !== "-" && at > 0 ? cut.slice(0, at) : cut).replace(/-+$/, "");
  }
  return s || "untitled";
};

// whereItGoes is where a document of kind k with this title would be filed
// today: its number is worked out when it's filed.
const whereItGoes = (k: DocKind, title: string) => {
  const d = new Date();
  const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  return k.where.replace("{date}", date).replace("{slug}", slug(title)).replace("{number}", "NNNN").replace(":", " · ");
};

// NewAsKind starts a page as one of the workspace's kinds of document: its
// agents are told the kind's guide, Draft it lays it out as the guide
// says, and File as… files it where the kind says.
export function NewAsKind({ kinds, busy, onStart, onClose }: { kinds: DocKind[]; busy: boolean; onStart: (title: string, kind: string) => void; onClose: () => void }) {
  const [kind, setKind] = useState(kinds[0]?.name ?? "");
  const [title, setTitle] = useState("");
  const k = kinds.find((x) => x.name === kind);
  const go = () => k && title.trim() && onStart(title.trim(), k.name);
  return (
    <Dialog title="Start a page as…" onClose={onClose} className="new-as" wide>
      <div className="kind-picks" role="radiogroup" aria-label="Kind of document">
        {kinds.map((x) => (
          <button key={x.name} role="radio" aria-checked={x.name === kind} className={`kind-pick${x.name === kind ? " on" : ""}`} onClick={() => setKind(x.name)} title={kindHint(x)}>
            {x.title}
          </button>
        ))}
      </div>
      <div className="file-as-fields">
        <label>
          <span>Title</span>
          <Input
            autoFocus
            value={title}
            placeholder={k ? `What the ${k.title} is about` : "Its title"}
            aria-label="Title"
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                go();
              }
            }}
          />
        </label>
      </div>
      {k && (
        <p className="dim file-as-what">
          Its agents are told how one's written{k.guide ? <> (<span className="mono">{k.guide.split("/").slice(-3).join("/")}</span>)</> : null}, and <em>Draft it</em> lays it out that way. Once it's written, <em>File as…</em> files it at <span className="mono">{whereItGoes(k, title)}</span>.
        </p>
      )}
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="primary" onClick={go} disabled={busy || !k || !title.trim()}>
          Start it <Keys k="⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
