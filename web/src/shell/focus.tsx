import { createContext, type DragEvent, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import { changeFocus, type FocusPin, type Plugin, type Workspace } from "../api";
import { useLoaded } from "../load";
import { thingsNamed } from "../refs/known";
import { prOf } from "../refs/prs";
import { GROUND, type Known, namedOf, type PageRef, refHref, refName, refNamed, thingAtPath } from "../refs/refs";
import { useKeys } from "../keys";
import { thingIcon } from "../refs/views";
import { Icon, type IconName } from "../ui/Icon";
import { humanError, toast } from "../ui/kit";
import { headlineStatus, Status } from "../ui/status";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";

// Focus: what you've pinned, of anything ws names, kept in the sidebar
// below its places (ops/focus.go). Each is drawn as it stands, and what
// ends (a mission landed, a conversation resolved) leaves it by itself.
// With nothing pinned there's nothing: not even the heading. You pin from
// a card, a page's Pin (or . on it), ⌘. in Find, or by dragging a link
// to it onto the sidebar; unpin from its row, or the same again.

interface FocusState {
  // supported: the daemon keeps pins (one from before them says nothing).
  supported: boolean;
  pins: FocusPin[];
  pinned: (name: string) => boolean;
  pin: (name: string, before?: string) => void;
  unpin: (name: string) => void;
  arrange: (names: string[]) => void;
}

const FocusContext = createContext<FocusState | undefined>(undefined);

export const useFocus = () => useContext(FocusContext);

// pinNameOf is what a ref is pinned as, or undefined for what isn't
// pinned: a file or folder on the default branch rather than its lines,
// an artifact rather than one version. A capsule's file, or a repo, isn't
// something to focus on.
export function pinNameOf(r: PageRef): string | undefined {
  switch (r.kind) {
    case "file":
      return r.worktree === GROUND
        ? refName({
            kind: "file",
            repo: r.repo,
            worktree: GROUND,
            path: r.path,
          })
        : undefined;
    case "dir":
      return r.worktree === GROUND ? refName(r) : undefined;
    case "artifact":
      return r.id;
    case "repo":
      return undefined;
  }
  return refName(r);
}

export function FocusProvider({ workspace, children }: { workspace?: Workspace; children: ReactNode }) {
  const told = workspace?.focus;
  // What a write answered, until the workspace's next read has it too.
  const [answered, setAnswered] = useState<FocusPin[]>();
  useEffect(() => setAnswered(undefined), [told]);
  const pins = answered ?? told ?? none;
  const value = useMemo<FocusState>(() => {
    const change = (c: Parameters<typeof changeFocus>[0]) =>
      changeFocus(c).then(
        (d) => setAnswered(d.focus),
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      );
    const pinned = (name: string) => pins.some((p) => p.name === name);
    return {
      supported: told !== undefined,
      pins,
      pinned,
      // before: where it goes, ahead of that pin; at the end without.
      pin: (name, before) =>
        changeFocus({ pin: name }).then(
          (d) => {
            const names = d.focus.map((p) => p.name).filter((n) => n !== name);
            const at = before ? names.indexOf(before) : -1;
            if (at < 0 || !d.focus.some((p) => p.name === name)) return setAnswered(d.focus);
            names.splice(at, 0, name);
            return change({ order: names });
          },
          (e: Error) => toast(humanError(e.message), { tone: "danger" }),
        ),
      unpin: (name) => {
        const order = pins.map((p) => p.name);
        change({ unpin: name }).then(() =>
          toast(`Unpinned ${name}`, {
            action: {
              label: "Undo",
              run: () => changeFocus({ pin: name }).then(() => change({ order })),
            },
          }),
        );
      },
      arrange: (names) => change({ order: names }),
    };
  }, [pins, told]);
  return <FocusContext.Provider value={value}>{children}</FocusContext.Provider>;
}

const none: FocusPin[] = [];

// pageRefAt is what the page at an address is, of what's pinned: a
// mission (any of its pages), a conversation, a pull request, a page, an
// artifact, a document on its space's default branch, a plugin's thing.
export function pageRefAt(path: string, search: string, known?: Known): PageRef | undefined {
  const seg = path
    .split("/")
    .slice(1)
    .map((s) => {
      try {
        return decodeURIComponent(s);
      } catch {
        return s;
      }
    });
  const page = (id?: string): PageRef | undefined => (id && /^p[1-9]\d*$/.test(id) ? { kind: "page", id } : undefined);
  switch (seg[0]) {
    case "missions":
      return seg[1] ? { kind: "mission", slug: seg[1] } : undefined;
    case "conversations":
      return seg[1] ? { kind: "talk", id: seg[1] } : undefined;
    case "reviews":
      return seg[1] && /^\d+$/.test(seg[2] ?? "") ? { kind: "pr", repo: seg[1], number: Number(seg[2]) } : undefined;
    case "pages":
      return page(seg[1]);
    case "artifacts":
      return seg[1] && seg[1] !== "compare" ? { kind: "artifact", id: seg[1] } : undefined;
    case "documents":
      if (seg[1] === "local") return page(seg[2]);
      return seg[1] && seg.length > 2 && !new URLSearchParams(search).has("in") ? { kind: "file", repo: seg[1], worktree: GROUND, path: seg.slice(2).join("/") } : undefined;
    case "x":
      return known ? thingAtPath(path, known) : undefined;
  }
  return undefined;
}

// FocusKeys is . anywhere: pin what the page is, or unpin it.
export function FocusKeys({ workspace, plugins }: { workspace?: Workspace; plugins?: Plugin[] }) {
  const focus = useFocus();
  const { pathname, search } = useLocation();
  const known = useSidebarKnown(workspace, plugins);
  useKeys({
    ".": () => {
      const r = focus?.supported ? pageRefAt(pathname, search, known) : undefined;
      const name = r && pinNameOf(r);
      if (!name || !focus) return;
      if (focus.pinned(name)) focus.unpin(name);
      else {
        focus.pin(name);
        toast(`Pinned ${name}`);
      }
    },
  });
  return null;
}

// PagePin is a page's own Pin, in its head: PinButton, for what the page is.
// compact is the pin alone, its words in its title: where a head's row
// has no room for more.
export function PagePin({ r, compact }: { r: PageRef; compact?: boolean }) {
  return <PinButton r={r} keyHint compact={compact} />;
}

// PinButton pins what a card's about, or unpins it: on a card, beside its
// way there.
export function PinButton({ r, keyHint, compact }: { r: PageRef; keyHint?: boolean; compact?: boolean }) {
  const focus = useFocus();
  const name = pinNameOf(r);
  if (!focus?.supported || !name) return null;
  const on = focus.pinned(name);
  return (
    <Button
      variant="ghost" size="small"
      className={`pin-button ${on ? " on" : ""}${compact ? " icon-button" : ""}`}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        if (on) focus.unpin(name);
        else focus.pin(name);
      }}
      title={(on ? "Take it out of your focus" : "Keep it in your focus, in the sidebar") + (keyHint ? " (.)" : "")}
      aria-pressed={on}
      aria-label={compact ? (on ? "Pinned" : "Pin") : undefined}
    >
      <Icon name="pin" size={12} />
      {!compact && (on ? "Pinned" : "Pin")}
    </Button>
  );
}

// FocusSection is the sidebar's Focus: your pins, a row each, nothing at
// all when there are none. A row's dragged to put them in order; a link
// to something pinnable, dragged from the page, is pinned where it's
// dropped (and while one's dragged, Focus is there to drop it on).
export function FocusSection({ workspace, plugins, path }: { workspace: Workspace; plugins?: Plugin[]; path: string }) {
  const focus = useFocus();
  const known = useSidebarKnown(workspace, plugins);
  // A row of Focus being moved, or a link from the page being brought in:
  // its pin's name either way.
  const [moving, setMoving] = useState<string>();
  const [incoming, setIncoming] = useState<string>();
  const [over, setOver] = useState<{ name: string; after: boolean }>();
  const knownNow = useRef(known);
  knownNow.current = known;
  useEffect(() => {
    const start = (e: globalThis.DragEvent) => {
      const a = (e.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!a || a.closest(".focus")) return;
      const url = new URL(a.href, window.location.href);
      if (url.origin !== window.location.origin) return;
      const r = pageRefAt(url.pathname, url.search, knownNow.current);
      const name = r && pinNameOf(r);
      if (name) setIncoming(name);
    };
    const end = () => {
      setIncoming(undefined);
      setOver(undefined);
    };
    window.addEventListener("dragstart", start);
    window.addEventListener("dragend", end);
    window.addEventListener("drop", end);
    return () => {
      window.removeEventListener("dragstart", start);
      window.removeEventListener("dragend", end);
      window.removeEventListener("drop", end);
    };
  }, []);
  const rows = (focus?.pins ?? []).flatMap((pin) => {
    const r = refNamed(pin.name, known);
    return r ? [{ pin, r }] : [];
  });
  const bringing = incoming && focus?.supported && !focus.pinned(incoming) ? incoming : undefined;
  if (!focus || (rows.length === 0 && !bringing)) return null;

  const names = rows.map((x) => x.pin.name);
  const reset = () => {
    setMoving(undefined);
    setIncoming(undefined);
    setOver(undefined);
  };
  // where is the pin a drop goes ahead of: the row it's over, or the one
  // after it; undefined for the end.
  const ahead = () => (over ? (over.after ? names[names.indexOf(over.name) + 1] : over.name) : undefined);
  const drop = (e: DragEvent) => {
    e.preventDefault();
    if (bringing) {
      focus.pin(bringing, ahead());
    } else if (moving && over && over.name !== moving) {
      const rest = names.filter((n) => n !== moving);
      const before = ahead();
      const at = before && before !== moving ? rest.indexOf(before) : rest.length;
      rest.splice(at < 0 ? rest.length : at, 0, moving);
      if (rest.join("\n") !== names.join("\n")) focus.arrange(rest);
    }
    reset();
  };
  const dragOver = (e: DragEvent) => {
    if (!moving && !bringing) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = bringing ? "copy" : "move";
  };

  return (
    <section className={`inbox focus${bringing ? " bringing" : ""}`} aria-label="Focus" onDragOver={dragOver} onDrop={drop}>
      <h2>
        Focus {rows.length > 0 && <Count>{rows.length}</Count>}
      </h2>
      {rows.map(({ pin, r }) => (
        <FocusRow
          key={pin.name}
          pin={pin}
          r={r}
          workspace={workspace}
          known={known}
          path={path}
          className={[moving === pin.name ? "dragging" : "", over?.name === pin.name && moving !== pin.name ? (over.after ? "drop-after" : "drop-before") : ""].join(" ")}
          onUnpin={() => focus.unpin(pin.name)}
          drag={{
            draggable: true,
            onDragStart: (e) => {
              e.dataTransfer.effectAllowed = "move";
              e.dataTransfer.setData("text/plain", pin.name);
              setMoving(pin.name);
            },
            onDragOver: (e) => {
              if (!moving && !bringing) return;
              const box = e.currentTarget.getBoundingClientRect();
              setOver({ name: pin.name, after: e.clientY > box.top + box.height / 2 });
            },
            onDragEnd: reset,
          }}
        />
      ))}
      {bringing && rows.length === 0 && <div className="focus-drop">Drop it here to pin it</div>}
    </section>
  );
}

// useSidebarKnown is what the sidebar names by: the workspace's, and how
// the plugins' things are named. (The pages' names are read beside them,
// with each repo's files, which a pin's row doesn't need.)
function useSidebarKnown(workspace: Workspace | undefined, plugins?: Plugin[]): Known {
  const named = JSON.stringify((plugins ?? []).map((p) => [p.name, p.refs ?? []]));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const things = useMemo(() => thingsNamed(plugins ?? []), [named]);
  return useMemo(
    () => ({
      repos: new Map(),
      root: workspace?.root,
      missions: workspace?.missions.map((m) => m.slug) ?? [],
      pages: (workspace?.pages ?? []).map((p) => p.id),
      talks: (workspace?.conversations ?? []).map((t) => t.id),
      artifacts: (workspace?.artifacts ?? []).map((a) => a.id),
      org: workspace?.org,
      defaultBranch: workspace?.defaultBranch,
      spaces: (workspace?.spaces ?? []).map((s) => ({ repo: s.repo, folder: s.folder })),
      things,
    }),
    [workspace, things],
  );
}

type Drag = Pick<React.HTMLAttributes<HTMLAnchorElement>, "draggable" | "onDragStart" | "onDragOver" | "onDragEnd">;

// FocusRow is a pin: how it stands, as an icon, and what it's called.
function FocusRow({
  pin,
  r,
  workspace,
  known,
  path,
  className,
  onUnpin,
  drag,
}: {
  pin: FocusPin;
  r: PageRef;
  workspace: Workspace;
  known?: Known;
  path: string;
  className: string;
  onUnpin: () => void;
  drag: Drag;
}) {
  const to = refHref(r, known);
  const { icon, title } = useStands(pin, r, workspace, known);
  const here = r.kind === "mission" ? path === to || path.startsWith(`${to}/`) : path === to;
  return (
    <Link to={to} className={`${here ? "active" : ""} ${className}`} title={`${title}${pin.state && r.kind === "thing" ? ` · ${pin.state}` : ""}`} {...drag}>
      {icon}
      <span>{title}</span>
      <button
        className="focus-unpin"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          onUnpin();
        }}
        title="Unpin"
        aria-label={`Unpin ${title}`}
      >
        <Icon name="x" size={12} />
      </button>
    </Link>
  );
}

const quiet = (name: IconName) => (
  <span className="status tone-quiet">
    <Icon name={name} />
  </span>
);

// useStands is how a pin stands, as an icon, and what it's called: from
// what the workspace says of it where it says (a mission's headline, a
// conversation waiting on you), else what the graph said.
function useStands(pin: FocusPin, r: PageRef, workspace: Workspace, known?: Known): { icon: ReactNode; title: string } {
  const pr = useLoaded(r.kind === "pr" ? () => prOf(r.repo, r.number) : null, [r.kind === "pr" ? `${r.repo}#${r.number}` : ""]).value;
  switch (r.kind) {
    case "mission": {
      const m = workspace.missions.find((x) => x.slug === r.slug);
      return {
        icon: m ? <Status s={headlineStatus(m.headline)} iconOnly /> : quiet("flag"),
        title: m?.title ?? pin.title ?? r.slug,
      };
    }
    case "talk": {
      const t = workspace.conversations.find((x) => x.id === r.id);
      const waits = !!t?.waiting && t.status === "open";
      return {
        icon: waits ? (
          <span className="status tone-attention" title="Waiting on you">
            <Icon name="reply" />
          </span>
        ) : (
          quiet("message")
        ),
        title: t?.title || t?.opening || pin.title || r.id,
      };
    }
    case "pr": {
      const tone = pr?.checks === "failure" ? "danger" : pr?.state === "needs-review" || pr?.state === "back-with-you" ? "attention" : "quiet";
      return {
        icon: (
          <span className={`status tone-${tone}`} title={pr?.checks === "failure" ? "Its checks fail" : undefined}>
            <Icon name="pr" />
          </span>
        ),
        title: `${r.repo}#${r.number} ${pr?.title ?? pin.title ?? ""}`.trim(),
      };
    }
    case "thing":
      return {
        icon: quiet(thingIcon(namedOf(r, known))),
        title: `${r.id} ${pin.title ?? ""}`.trim(),
      };
    case "page":
      return {
        icon: quiet("file"),
        title: workspace.pages?.find((p) => p.id === r.id)?.title || pin.title || r.id,
      };
    case "artifact":
      return {
        icon: quiet("artifact"),
        title: workspace.artifacts?.find((a) => a.id === r.id)?.title || pin.title || r.id,
      };
    case "dir":
      return { icon: quiet("layers"), title: pin.title || `${r.path}/` };
    case "file":
      return {
        icon: quiet("file"),
        title: pin.title || r.path.split("/").pop() || r.path,
      };
  }
  return { icon: quiet("circle"), title: pin.title || pin.name };
}
