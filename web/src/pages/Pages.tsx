import { lazy, type ReactNode, Suspense, useState } from "react";
import { useNavigate } from "react-router";
import { type DocKind, kindHint, newPage, pageAddress, type PageInfo } from "../api";
import type { Back } from "./Page";
import { toast } from "../ui/toast";
import { Loading } from "../ui/Loading";
import { humanError } from "../ui/strings";
import { Keys } from "../ui/Keys";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { useThingMenu } from "../shell/ThingMenu";
import { PageField } from "./PageField";
import { PageFields } from "./PageFields";
import { PageFileWhat } from "./PageFileWhat";
import { PageKindPick } from "./PageKindPick";
import { PageKindPicks } from "./PageKindPicks";
import { PagesRow } from "./PagesRow";
import { Text } from "../ui/Text";

// Pages: documents you write here, before or beside any mission. A line
// that starts with >> asks for something (a table of the routes, what
// calls this), and an agent fills it in while you write on. They're
// listed in Documents (documents/), where each will live.

export const pageKeys: [string, string][] = [
  [">> then ⏎", "Ask an agent to fill in here"],
  [">? then ⏎", "Ask a question, answered beside the page"],
  ["mod+⏎", "In a cell: change or see sources. Elsewhere: change the text"],
  ["esc", "Close a cell's panel"],
];

// The page itself (the editor, and so ProseMirror) loads with the first
// page you open.
const Inner = lazy(() => import("./Page"));

export function PagePage({ id, back }: { id?: string; back?: Back }) {
  return (
    <Suspense fallback={<Loading what="Opening page…" />}>
      <Inner id={id} back={back} />
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

// PageRow is a page in a list, drawn as a PagesRow: its title, its kind,
// the file it's tied to, its cells, and when it last changed. note says
// more of it, after its chips (where it's filed, say).
export function PageRow({ page: p, kinds, selected, onHover, note }: { page: PageInfo; kinds: DocKind[]; selected: boolean; onHover: () => void; note?: ReactNode }) {
  const menu = useThingMenu();
  const kind = p.kind ? kinds.find((k) => k.name === p.kind) : undefined;
  return (
    <PagesRow
      id={p.id}
      title={p.title}
      to={pageAddress(p)}
      kind={p.kind ? { title: kind?.title ?? p.kind, hint: kind ? kindHint(kind) : `Kind no longer exists (${p.kind})` } : undefined}
      file={p.source && { name: p.source.ref.replace(/^[^:]*:/, "").split("/").pop() ?? "", ref: p.source.ref }}
      filling={p.filling}
      cells={p.cells}
      stale={p.stale}
      updated={p.updated}
      note={note}
      selected={selected}
      onHover={onHover}
      onContextMenu={menu(pageAddress(p), p.title)}
    />
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
    <Dialog title="Start a page as…" onClose={onClose} wide>
      <PageKindPicks>
        {kinds.map((x) => (
          <PageKindPick key={x.name} on={x.name === kind} onClick={() => setKind(x.name)} title={kindHint(x)}>
            {x.title}
          </PageKindPick>
        ))}
      </PageKindPicks>
      <PageFields>
        <PageField label="Title">
          <Input
            autoFocus
            value={title}
            placeholder={k ? `What the ${k.title} is about` : "Title"}
            aria-label="Title"
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                go();
              }
            }}
          />
        </PageField>
      </PageFields>
      {k && (
        <PageFileWhat>
          Agents follow this kind's guide
          {k.guide ? <> (<Text mono>{k.guide.split("/").slice(-3).join("/")}</Text>)</> : null}, and{" "}
          <em>Draft</em> lays out the page to match. When written, <em>File as…</em> files to{" "}
          <Text mono>{whereItGoes(k, title)}</Text>.
        </PageFileWhat>
      )}
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="primary" onClick={go} disabled={busy || !k || !title.trim()} title="Start page" keys="⏎">
          Start <Keys k="⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
