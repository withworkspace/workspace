import { V1 } from "@withworkspace/ui-lib/protocol";
import type { Anchor } from "../ui/FloatingCard";
import { type Hovering, hovering } from "./hover";
import { apiBase, frameBase } from "./paths";

// ws's own UI, for a plugin's page. Some of what a page wants shown is
// ws's to know or to do (a ref's card), and none of it can a frame draw
// past its own edge. So ws has UIs of its own that a page asks for by
// name (ws:ui-open): what each is, how it behaves and what it does is
// defined here in ws, never by the plugin. The page says which, what it's
// about, and where on its page it's asked from (a box), and ws draws it
// by there, over the frame (Uis/FrameUiLayer.tsx). What comes of one that
// ws closes goes back to the page (ws:ui-closed).
//
// A new UI is a name in the frame protocol's V1.Uis, with what it's given
// and what comes of it; its kind here; and what draws it, in
// Uis/FrameUi.tsx. Both lists are typed by the protocol's names, so
// neither can be forgotten.

// Asked is one of ws's UIs as a page asked for it.
export interface Asked {
  // The page's number for the asking.
  id: number;
  // Which UI.
  ui: string;
  // Where on the page it's asked from, in the frame's own viewport.
  box?: V1.Box;
  // What it's given.
  with: Record<string, unknown>;
}

// UiKind is how one of ws's UIs is had.
export interface UiKind {
  // hover: it comes a moment after it's asked for and goes a moment after
  // it's closed, stays while the pointer's on it, and is the only one of
  // its sort up (a card). open: it's there from when it's asked for until
  // ws is done with it or the page closes it (a dialog).
  as: "hover" | "open";
  // Why what the page gave isn't what it takes, if it isn't: a sentence
  // for whoever's writing the page. plugin is whose page asked.
  refuses?(asked: Asked, plugin: string): string | undefined;
}

// The UIs ws has for a plugin's page, each by its name in the protocol.
export const kinds: Readonly<Record<V1.UiName, UiKind>> = {
  // A ref's card, by its words, while the pointer's on them.
  "ref-card": {
    as: "hover",
    refuses: (asked) => (typeof asked.with.ref === "string" && asked.with.ref && asked.box ? undefined : "ref-card needs a ref and the box of its text."),
  },
  // One of the plugin's own things put in a repo, and saved there, by
  // the button that asked (Uis/ThingPromoteUi.tsx, Uis/ThingSaveUi.tsx).
  "thing-promote": { as: "open", refuses: (asked) => aboutThing("thing-promote", asked) },
  "thing-save": { as: "open", refuses: (asked) => aboutThing("thing-save", asked) },
  // A picture over the whole window, in ws's viewer (Uis/PictureUi.tsx).
  // It's drawn in ws's page, so it's only ever an address ws's page may
  // show a picture from, and one on ws's is the plugin's own.
  picture: { as: "open", refuses: (asked, plugin) => pictureRefused(asked, plugin) },
};

// pictureRefused is why a picture asked for can't be shown, if it can't:
// the protocol's rule, for this plugin, on this ws.
export function pictureRefused(asked: Asked, plugin: string, origin = globalThis.location?.origin ?? ""): string | undefined {
  const { src, title } = asked.with;
  if (typeof src !== "string") return "picture needs an address: { src }.";
  if (title !== undefined && typeof title !== "string") return "picture's title must be text.";
  return V1.pictureRefused(src, origin, { api: apiBase(plugin), base: frameBase(plugin) });
}

// aboutThing is why a UI about one of a plugin's things wasn't given one,
// if it wasn't: a thing is its kind, and the plugin's name for it.
function aboutThing(ui: string, asked: Asked): string | undefined {
  const given = (v: unknown) => typeof v === "string" && v.trim() !== "";
  return given(asked.with.kind) && given(asked.with.ref) ? undefined : `${ui} needs the thing's kind and the plugin's ref for it: { kind: "sketch", ref: "sketch-12" }.`;
}

// FrameUis is the UIs one frame's page has asked for: what the page says
// of them, and what ws draws and says back.
export interface FrameUis {
  // The UIs there are, by name: what the page is told ws has as it
  // connects, so it asks for nothing else.
  readonly names: readonly string[];
  // The page asks for one. What comes back is why it can't be shown, if
  // it can't: ws has none of that name, or it wasn't given what it takes.
  open(asked: Asked): string | undefined;
  // The page takes one back.
  close(id: number): void;
  // The page went: all it asked for goes with it, and nobody's told.
  forget(): void;

  // What's shown on hover, if anything is.
  hovered(): Asked | undefined;
  // What's open, in the order it was asked for.
  opened(): readonly Asked[];
  // hear runs fn when either changes, and returns what stops it.
  hear(fn: () => void): () => void;
  // The pointer's on what's shown on hover, and has left it.
  stay(): void;
  leave(): void;
  // What's shown on hover goes at once: ws moved the page.
  unhover(): void;
  // ws is done with one that's open: it's gone, and the page is told
  // what came of it (nothing, when it was only dismissed).
  done(id: number, result?: unknown): void;
}

const sameBox = (a?: V1.Box, b?: V1.Box) => a === b || (!!a && !!b && a.x === b.x && a.y === b.y && a.width === b.width && a.height === b.height);
// The same UI, about the same thing, asked from the same place.
const sameAsking = (a: Asked, b: Asked) => a.ui === b.ui && sameBox(a.box, b.box) && JSON.stringify(a.with) === JSON.stringify(b.with);

// frameUis keeps one frame's. closed tells the page that ws closed one,
// and with what. has is the UIs there are: ws's own, unless a test says.
// plugin is whose page it is.
export function frameUis(closed: (id: number, result: unknown) => void, has: Readonly<Record<string, UiKind>> = kinds, plugin = ""): FrameUis {
  const card: Hovering<Asked> = hovering(sameAsking);
  // The asking that's hovering, as the page has it: the one it closes.
  let hoverId: number | undefined;
  let opened: readonly Asked[] = [];
  const heard = new Set<() => void>();
  const changed = () => {
    for (const fn of [...heard]) fn();
  };
  card.hear(changed);
  const take = (id: number) => {
    if (!opened.some((o) => o.id === id)) return false;
    opened = opened.filter((o) => o.id !== id);
    changed();
    return true;
  };

  return {
    names: Object.keys(has),
    open(asked) {
      const kind = Object.hasOwn(has, asked.ui) ? has[asked.ui] : undefined;
      if (!kind) return `Workspace has no UI called ${asked.ui}. Update Workspace.`;
      const why = kind.refuses?.(asked, plugin);
      if (why) return why;
      if (kind.as === "hover") {
        hoverId = asked.id;
        card.over(asked);
      } else {
        opened = [...opened.filter((o) => o.id !== asked.id), asked];
        changed();
      }
      return undefined;
    },
    close(id) {
      if (id === hoverId) {
        hoverId = undefined;
        card.over(undefined);
      } else take(id);
    },
    forget() {
      hoverId = undefined;
      card.clear();
      if (!opened.length) return;
      opened = [];
      changed();
    },
    hovered: card.shown,
    opened: () => opened,
    hear(fn) {
      heard.add(fn);
      return () => void heard.delete(fn);
    },
    stay: card.stay,
    leave: card.leave,
    unhover: card.clear,
    done(id, result) {
      if (take(id)) closed(id, result);
    },
  };
}

// anchorIn is a box in a frame's page as somewhere in ws's window, for
// what ws draws to be placed by: the frame's corner, and the box from it.
// It's read each time it's asked, so it's right wherever the frame is by
// then (a pane docked beside it since), and it's there for as long as the
// frame is.
export function anchorIn(frame: () => Pick<Element, "isConnected" | "getBoundingClientRect" | "clientLeft" | "clientTop"> | null | undefined, box: V1.Box): Anchor {
  return {
    get isConnected() {
      return !!frame()?.isConnected;
    },
    getBoundingClientRect() {
      const f = frame();
      const at = f?.getBoundingClientRect();
      // The page starts inside the frame's border, where it has one.
      const left = (at?.left ?? 0) + (f?.clientLeft ?? 0) + box.x;
      const top = (at?.top ?? 0) + (f?.clientTop ?? 0) + box.y;
      return { left, top, right: left + box.width, bottom: top + box.height };
    },
  };
}
