import { useEffect } from "react";

// useKeyboarding marks the page while you move by keys, so the keyboard's
// cursor shows; moving the mouse hides it again.
export function useKeyboarding() {
  useEffect(() => {
    const nav = new Set(["j", "k", "ArrowUp", "ArrowDown", "g", "G", "n", "p"]);
    const onKey = (e: KeyboardEvent) => nav.has(e.key) && !e.metaKey && !e.ctrlKey && document.body.classList.add("keyboarding");
    const onMouse = () => document.body.classList.remove("keyboarding");
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousemove", onMouse);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousemove", onMouse);
    };
  }, []);
}
