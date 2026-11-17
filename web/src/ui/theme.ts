import { useSyncExternalStore } from "react";

// The theme: your choice in this browser (system, light or dark), shown as
// html[data-theme]. public/theme.js sets it before the page draws; this
// keeps it in step after (a change of choice, or of the system's).

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

let snapshot = { choice, dark: isDark() };

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useTheme(): { choice: ThemeChoice; dark: boolean } {
  return useSyncExternalStore(subscribe, () => {
    if (snapshot.choice !== choice || snapshot.dark !== isDark()) snapshot = { choice, dark: isDark() };
    return snapshot;
  });
}
