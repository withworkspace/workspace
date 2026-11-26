import { useMemo } from "react";
import { changeFocus, type FocusPin } from "../api";
import { toast } from "../ui/toast";
import { humanError } from "../ui/strings";
import { useWorkspaceIfRead, useWorkspaceStore } from "../workspace/store";
import { pinsAnswered, unpinNamed } from "./pins";

// Focus: what you've pinned, of anything ws names, kept in the sidebar
// below its places (ops/focus.go). Each is drawn as it stands, and what
// ends (a mission landed, a conversation resolved) leaves it by itself.
// With nothing pinned there's nothing: not even the heading. You pin from
// a card, a page's Pin (or . on it), ⌘. in Find, or by dragging a link
// to it onto the sidebar; unpin from its row, or the same again.

export interface FocusState {
  // supported: the daemon keeps pins (one from before them says nothing).
  supported: boolean;
  pins: FocusPin[];
  pinned: (name: string) => boolean;
  pin: (name: string, before?: string) => void;
  unpin: (name: string) => void;
  arrange: (names: string[]) => void;
}

// useFocus is your pins, and how they're changed: from anywhere. The
// pins are the workspace's (workspace/store.ts); a write of them is
// answered with how they are then, which they are until the workspace is
// next read.
export function useFocus(): FocusState {
  const told = useWorkspaceIfRead((w) => w.focus);
  const answered = useWorkspaceStore((s) => s.focusAnswered);
  const pins = answered ?? told ?? none;
  return useMemo<FocusState>(() => {
    const setAnswered = pinsAnswered;
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
      unpin: (name) => void unpinNamed(name).catch((e: Error) => toast(humanError(e.message), { tone: "danger" })),
      arrange: (names) => change({ order: names }),
    };
  }, [pins, told]);
}

const none: FocusPin[] = [];
