import { useSyncExternalStore } from "react";

// The theme: your choice in this browser (system, light or dark), shown as
// html[data-theme], and how deep the dark theme goes (0 to 100, towards
// black for an OLED screen), shown as --dark-depth (0 to 1) on <html>.
// public/theme.js sets both before the page draws; this keeps them in
// step after (a change of choice, or of the system's).

export type ThemeChoice = "system" | "light" | "dark";

const media = window.matchMedia("(prefers-color-scheme: dark)");
const listeners = new Set<() => void>();

function read(): ThemeChoice {
  try {
    const c = localStorage.getItem("ws-theme");
    return c === "light" || c === "dark" ? c : "system";
  } catch {
    return "system";
  }
}

let choice = read();

function readDepth(): number {
  try {
    const n = Number(localStorage.getItem("ws-depth"));
    return Number.isFinite(n) ? Math.min(Math.max(Math.round(n), 0), 100) : 0;
  } catch {
    return 0;
  }
}

let depth = readDepth();

// showDepth puts a depth on the page, without keeping it: for the slider
// as it's dragged.
export function showDepth(n: number) {
  const style = document.documentElement.style;
  if (n > 0) style.setProperty("--dark-depth", String(n / 100));
  else style.removeProperty("--dark-depth");
}

// setDepth keeps how deep the dark theme goes in this browser, and shows it.
export function setDepth(n: number) {
  depth = Math.min(Math.max(Math.round(n), 0), 100);
  try {
    if (depth === 0) localStorage.removeItem("ws-depth");
    else localStorage.setItem("ws-depth", String(depth));
  } catch {
    // storage off: it lasts as long as the page
  }
  showDepth(depth);
  listeners.forEach((l) => l());
}

const isDark = () => choice === "dark" || (choice === "system" && media.matches);

function apply() {
  document.documentElement.setAttribute("data-theme", isDark() ? "dark" : "light");
  listeners.forEach((l) => l());
}

media.addEventListener("change", () => choice === "system" && apply());

export function setTheme(c: ThemeChoice) {
  choice = c;
  try {
    if (c === "system") localStorage.removeItem("ws-theme");
    else localStorage.setItem("ws-theme", c);
  } catch {
    // storage off: the choice lasts as long as the page
  }
  apply();
}

let snapshot = { choice, dark: isDark(), depth };

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useTheme(): { choice: ThemeChoice; dark: boolean; depth: number } {
  return useSyncExternalStore(subscribe, () => {
    if (snapshot.choice !== choice || snapshot.dark !== isDark() || snapshot.depth !== depth) snapshot = { choice, dark: isDark(), depth };
    return snapshot;
  });
}
