import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { getSpaces, pageAddress, type SpaceActivity, spaceTo } from "../api";
import { keyOf, useKeymap } from "../keymap/keymap";
import { useCommand, useCommands } from "../keymap/useCommands";
import { documentsKeys } from "./keys";
import { useLoaded } from "../load";
import { NewAsKind, PageRow, useStartPage } from "../pages/Pages";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Loading } from "../ui/Loading";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { DocumentsAcross } from "./DocumentsAcross";
import { DocumentsEmpty } from "./DocumentsEmpty";
import { DocumentsFrame } from "./DocumentsFrame";
import { DocumentsGrid } from "./DocumentsGrid";
import { DocumentsSpaceCards } from "./DocumentsSpaceCards";
import { DocumentsSub } from "./DocumentsSub";
import { acrossSpaces } from "./happening";
import { InFlight } from "./InFlight";
import { Lately } from "./Lately";
import { placePages } from "./place";
import { Results } from "./Results";
import { SearchBox } from "./SearchBox";
import { SpaceCard } from "./SpaceCard";
import { hitTo, useDocSearch } from "./useDocSearch";
import { useSpaceDetails } from "./useSpaceDetails";

// Documents: where the workspace's writing is read. Local is the pages
// that aren't in a space (written here, on this machine); each space is a
// repo of documents, or a folder of one, read from its default branch. A
// page that will live in a space is listed there, as your draft.
//
// Documents takes the window's width: what changed and what's in flight
// across every space on top, each space as a card below (its kinds and
// how it stands), and Local beside them. A space's page is SpacePage.
//
// DocumentsPage is what Documents knows and does: the spaces and the
// pages, what's typed into its search and what that found, the keys'
// cursor and starting a page. What it draws are the Documents… parts.

// A row the cursor can be on: a page, or a space.
type Target = { id: string; to: string };

export function DocumentsPage() {
  const workspace = useWorkspace();
  useTitle("Documents");
  const navigate = useNavigate();
  // Answered at once from what ws keeps: read again when it's read a
  // space again and found something new (the workspace's spacesSeen).
  const { value, error, reload } = useLoaded(getSpaces, [workspace.spacesSeen], { keep: true });
  const spaces = useMemo(() => value?.spaces ?? [], [value]);
  const kinds = useMemo(() => workspace.kinds ?? [], [workspace.kinds]);
  const placed = useMemo(() => placePages(workspace.pages ?? [], spaces, kinds), [workspace.pages, spaces, kinds]);
  const local = placed.filter((p) => !p.repo);
  const details = useSpaceDetails(spaces, workspace.spacesSeen);
  const across = useMemo(() => {
    const activity = new Map<string, SpaceActivity>();
    for (const [repo, d] of details) if (d.activity) activity.set(repo, d.activity);
    return acrossSpaces(spaces, activity);
  }, [spaces, details]);
  const spaceOf = (repo: string) => (spaces.length > 1 ? spaces.find((s) => s.repo === repo) : undefined);
  const { start, starting } = useStartPage();
  const [asKind, setAsKind] = useState(false);
  const [index, setIndex] = useState(0);
  // Searching: what's found in place of the lists.
  const [q, setQ] = useState("");
  const [kind, setKind] = useState("");
  const [stand, setStand] = useState("");
  const search = useRef<HTMLInputElement>(null);
  const found = useDocSearch(q, kind, stand);
  const searching = q.trim() !== "";
  const [hit, setHit] = useState(0);
  useEffect(() => setHit(0), [found.hits]);
  const moveHit = (i: number) => {
    const n = Math.min(Math.max(i, 0), (found.hits?.length ?? 1) - 1);
    setHit(n);
    document.getElementById(`hit-${n}`)?.scrollIntoView({ block: "nearest" });
  };
  const searchKeys = (e: React.KeyboardEvent) => {
    const go: Record<string, () => void> = {
      ArrowDown: () => moveHit(hit + 1),
      ArrowUp: () => moveHit(hit - 1),
      Enter: () => found.hits?.[hit] && navigate(hitTo(found.hits[hit])),
      Escape: () => (q ? setQ("") : search.current?.blur()),
    };
    if (go[e.key]) {
      e.preventDefault();
      go[e.key]();
    }
  };

  // The cursor goes down the spaces, then Local, as they read.
  const targets: Target[] = [
    ...spaces.map((s) => ({ id: `space-${s.repo}`, to: spaceTo(s.repo) })),
    ...local.map((p) => ({ id: `page-${p.page.id}`, to: pageAddress(p.page) })),
  ];
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), targets.length - 1);
    setIndex(n);
    document.getElementById(targets[n]?.id ?? "")?.scrollIntoView({ block: "nearest" });
  };
  const open = () => targets[index] && navigate(targets[index].to);
  // Starting a page, each way with its key: the buttons that do it are
  // given these, and say the key.
  const newPage = useCommand(documentsKeys.newPage, () => start());
  const newAsKind = useCommand(documentsKeys.newAsKind, () => setAsKind(true), { enabled: kinds.length > 0 });
  const openHit = () => found.hits?.[hit] && navigate(hitTo(found.hits[hit]));
  useCommands(documentsKeys, {
    search: () => search.current?.focus(),
    nextHit: searching && (() => moveHit(hit + 1)),
    prevHit: searching && (() => moveHit(hit - 1)),
    openHit: searching && openHit,
    clearSearch: searching && (() => setQ("")),
    next: !searching && (() => move(index + 1)),
    prev: !searching && (() => move(index - 1)),
    open: !searching && open,
  });
  const map = useKeymap();
  const kbd = (k: string | undefined) => k && <Keys k={k} />;
  const filling = local.reduce((n, p) => n + (p.page.filling ? 1 : 0), 0);

  return (
    <DocumentsFrame>
      <PageHead
        title="Documents"
        actions={
          <>
            {kinds.length > 0 && (
              <Button
                onClick={newAsKind}
                disabled={starting}
                title="New page from a document kind"
              >
                New as… {kbd(keyOf(documentsKeys.newAsKind, map))}
              </Button>
            )}
            <Button variant="primary" onClick={newPage} disabled={starting} title="New page">
              <Icon name="plus" /> New page {kbd(keyOf(documentsKeys.newPage, map))}
            </Button>
          </>
        }
      />
      {asKind && <NewAsKind kinds={kinds} busy={starting} onStart={(title, kind) => start(title, kind)} onClose={() => setAsKind(false)} />}
      {spaces.length > 0 && <SearchBox q={q} setQ={setQ} inputRef={search} spaces={spaces} kind={kind} setKind={setKind} state={stand} setState={setStand} onKey={searchKeys} />}
      {searching ? (
        <Results hits={found.hits} more={found.more} error={found.error} index={hit} setIndex={setHit} />
      ) : (
        <>
          {(across.lately.length > 0 || across.inFlight.length > 0) && (
            <DocumentsAcross>
              <Lately
                lately={across.lately}
                spaceOf={spaceOf}
                label={
                  <>
                    Recent changes
                    {spaces.length > 1 && <DocumentsSub> · across {spaces.length} spaces</DocumentsSub>}
                  </>
                }
              />
              <InFlight flights={across.inFlight} spaceOf={spaceOf} />
            </DocumentsAcross>
          )}
          <DocumentsGrid
            spaces={
              <>
                <GroupHead count={spaces.length}>Spaces</GroupHead>
                {error && !value && <ErrorCard title="Could not load spaces" error={error} retry={reload} />}
                {value?.problem && <Banner tone="attention">May be out of date: {value.problem}</Banner>}
                {!value && !error && <Loading what="Loading spaces…" />}
                {value && spaces.length === 0 && (
                  <Empty icon="file" title="No document spaces">
                    Show a repo of documents (ADRs, proposals, guides) here: set <code>documents = true</code>{" "}
                    under its <code>[repos.&lt;name&gt;]</code> in <code>ws.toml</code>, or name it as{" "}
                    <code>[missions] records</code>.
                  </Empty>
                )}
                {spaces.length > 0 && (
                  <DocumentsSpaceCards few={spaces.length <= 2}>
                    {spaces.map((s, i) => (
                      <SpaceCard
                        key={s.repo}
                        id={`space-${s.repo}`}
                        space={s}
                        detail={details.get(s.repo)}
                        drafts={placed.filter((p) => p.repo === s.repo).length}
                        selected={i === index}
                        onHover={() => i !== index && setIndex(i)}
                      />
                    ))}
                  </DocumentsSpaceCards>
                )}
              </>
            }
            local={
              <>
                <GroupHead title="Pages on this machine, not in a space: only you see them. Use File as… to add one to a space.">
                  Local <Count>{local.length}</Count>
                  {filling > 0 && <DocumentsSub>· {filling} filling</DocumentsSub>}
                </GroupHead>
                {local.length === 0 ? (
                  <DocumentsEmpty>
                    No local pages yet:{" "}
                    <Button variant="link" onClick={newPage} title="New page">
                      start one
                    </Button>
                    .
                  </DocumentsEmpty>
                ) : (
                  <Rows role="listbox" aria-label="Local pages">
                    {local.map((p, j) => {
                      const i = spaces.length + j;
                      return (
                        <PageRow
                          key={p.page.id}
                          page={p.page}
                          kinds={kinds}
                          selected={i === index}
                          onHover={() => i !== index && setIndex(i)}
                          note={
                            p.page.source && (
                              <Chip tone="quiet" title="File not in a document space">
                                {p.page.source.ref.split(":")[0]}
                              </Chip>
                            )
                          }
                        />
                      );
                    })}
                  </Rows>
                )}
              </>
            }
          />
        </>
      )}
    </DocumentsFrame>
  );
}
