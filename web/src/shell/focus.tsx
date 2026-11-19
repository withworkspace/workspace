import { createContext, type DragEvent, type ReactNode, useContext, useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { changeFocus, type FocusPin, type Plugin, type Workspace } from "../api";
import { useLoaded } from "../load";
import { thingsNamed } from "../refs/known";
import { prOf } from "../refs/prs";
import { GROUND, type Known, namedOf, type PageRef, refHref, refName, refNamed } from "../refs/refs";
import { thingIcon } from "../refs/views";
import { Icon, type IconName } from "../ui/Icon";
import { humanError, toast } from "../ui/kit";
import { headlineStatus, Status } from "../ui/status";

// Focus: what you've pinned, of anything ws names, kept in the sidebar
// below its places (ops/focus.go). Each is drawn as it stands, and what
// ends (a mission landed, a conversation resolved) leaves it by itself.
// With nothing pinned there's nothing: not even the heading. You pin from
// a card, or ⌘. in Find; unpin from its row, or the same again.

interface FocusState {
  // supported: the daemon keeps pins (one from before them says nothing).
  supported: boolean;
  pins: FocusPin[];
  pinned: (name: string) => boolean;
  pin: (name: string) => void;
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
      pin: (name) => change({ pin: name }),
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

// PinButton pins what a card's about, or unpins it: on a card, beside its
// way there.
export function PinButton({ r }: { r: PageRef }) {
  const focus = useFocus();
  const name = pinNameOf(r);
  if (!focus?.supported || !name) return null;
  const on = focus.pinned(name);
  return (
    <button
      className={`ghost small pin-button${on ? " on" : ""}`}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        if (on) focus.unpin(name);
        else focus.pin(name);
      }}
      title={on ? "Take it out of your focus" : "Keep it in your focus, in the sidebar"}
      aria-pressed={on}
    >
      <Icon name="pin" size={12} />
      {on ? "Pinned" : "Pin"}
    </button>
  );
}

// FocusSection is the sidebar's Focus: your pins, a row each, nothing at
// all when there are none. A row's dragged to put them in order.
export function FocusSection({ workspace, plugins, path }: { workspace: Workspace; plugins?: Plugin[]; path: string }) {
  const focus = useFocus();
  const known = useSidebarKnown(workspace, plugins);
  const [dragging, setDragging] = useState<string>();
  const [over, setOver] = useState<{ name: string; after: boolean }>();
  const rows = (focus?.pins ?? []).flatMap((pin) => {
    const r = refNamed(pin.name, known);
    return r ? [{ pin, r }] : [];
  });
  if (!focus || rows.length === 0) return null;

  const names = rows.map((x) => x.pin.name);
  const drop = (e: DragEvent) => {
    e.preventDefault();
    if (!dragging || !over || over.name === dragging) return reset();
    const rest = names.filter((n) => n !== dragging);
    const at = rest.indexOf(over.name) + (over.after ? 1 : 0);
    rest.splice(at, 0, dragging);
    reset();
    if (rest.join("\n") !== names.join("\n")) focus.arrange(rest);
  };
  const reset = () => {
    setDragging(undefined);
    setOver(undefined);
  };

  return (
    <section className="inbox focus" aria-label="Focus">
      <h2>
        Focus <span className="count">{rows.length}</span>
      </h2>
      {rows.map(({ pin, r }) => (
        <FocusRow
          key={pin.name}
          pin={pin}
          r={r}
          workspace={workspace}
          known={known}
          path={path}
          className={[dragging === pin.name ? "dragging" : "", over?.name === pin.name && dragging !== pin.name ? (over.after ? "drop-after" : "drop-before") : ""].join(" ")}
          onUnpin={() => focus.unpin(pin.name)}
          drag={{
            draggable: true,
            onDragStart: (e) => {
              e.dataTransfer.effectAllowed = "move";
              e.dataTransfer.setData("text/plain", pin.name);
              setDragging(pin.name);
            },
            onDragOver: (e) => {
              if (!dragging) return;
              e.preventDefault();
              const box = e.currentTarget.getBoundingClientRect();
              setOver({
                name: pin.name,
                after: e.clientY > box.top + box.height / 2,
              });
            },
            onDrop: drop,
            onDragEnd: reset,
          }}
        />
      ))}
    </section>
  );
}

// useSidebarKnown is what the sidebar names by: the workspace's, and how
// the plugins' things are named. (The pages' names are read beside them,
// with each repo's files, which a pin's row doesn't need.)
function useSidebarKnown(workspace: Workspace, plugins?: Plugin[]): Known {
  const named = JSON.stringify((plugins ?? []).map((p) => [p.name, p.refs ?? []]));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const things = useMemo(() => thingsNamed(plugins ?? []), [named]);
  return useMemo(
    () => ({
      repos: new Map(),
      root: workspace.root,
      missions: workspace.missions.map((m) => m.slug),
      pages: (workspace.pages ?? []).map((p) => p.id),
      talks: workspace.conversations.map((t) => t.id),
      artifacts: (workspace.artifacts ?? []).map((a) => a.id),
      org: workspace.org,
      defaultBranch: workspace.defaultBranch,
      spaces: (workspace.spaces ?? []).map((s) => ({
        repo: s.repo,
        folder: s.folder,
      })),
      things,
    }),
    [workspace, things],
  );
}

type Drag = Pick<React.HTMLAttributes<HTMLAnchorElement>, "draggable" | "onDragStart" | "onDragOver" | "onDrop" | "onDragEnd">;

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
