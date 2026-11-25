import { changeFocus, type FocusPin } from "../api";
import { toast } from "../ui/toast";
import { useWorkspaceStore } from "../workspace/store";

// Writes of your pins (Focus, shell/useFocus.ts), from wherever one's
// made: the sidebar, a card's Pin, Find, a row's menu. A write is
// answered with how the pins are then, which is how the workspace has
// them from then on, so what's drawn of them and what's offered (Pin it,
// or Unpin it) agree without the workspace being read again.

// pinsAnswered keeps the pins as a write of them answered.
export const pinsAnswered = (focus: FocusPin[]) => useWorkspaceStore.setState((s) => ({ focusAnswered: focus, workspace: s.workspace && { ...s.workspace, focus } }));

// pinsNow is your pins as they stand: as last answered, or as the
// workspace was last read.
const pinsNow = (): FocusPin[] => {
  const s = useWorkspaceStore.getState();
  return s.focusAnswered ?? s.workspace?.focus ?? [];
};

// pinNamed pins what ws calls name, at the end of your pins.
export const pinNamed = (name: string) => changeFocus({ pin: name }).then((d) => pinsAnswered(d.focus));

// unpinNamed takes it out of your pins, and says so with a way back: it's
// pinned again where it was.
export function unpinNamed(name: string) {
  const order = pinsNow().map((p) => p.name);
  return changeFocus({ unpin: name }).then((d) => {
    pinsAnswered(d.focus);
    toast(`Unpinned ${name}`, {
      action: {
        label: "Undo",
        run: () =>
          changeFocus({ pin: name })
            .then(() => changeFocus({ order }))
            .then((d) => pinsAnswered(d.focus)),
      },
    });
  });
}
