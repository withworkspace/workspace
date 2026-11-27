import { useEffect } from "react";

// useKeyboarding marks the page while you move by keys, so the keyboard's
// cursor shows; moving the mouse hides it again. A command that moves the
// cursor (j, k, or the keys you've set for them) marks it as it's done
// (keymap/dispatch.ts). The arrows mark it here as well: they're also a
// part's own keys (a tree's, a list's) that aren't commands, and are the
// same keys whatever you've set.
export function useKeyboarding() {
  useEffect(() => {
    const arrows = new Set(["ArrowUp", "ArrowDown"]);
    const onKey = (e: KeyboardEvent) => arrows.has(e.key) && !e.metaKey && !e.ctrlKey && document.body.classList.add("keyboarding");
    const onMouse = () => document.body.classList.remove("keyboarding");
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousemove", onMouse);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousemove", onMouse);
    };
  }, []);
}
