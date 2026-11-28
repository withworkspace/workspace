import { useEffect } from "react";

// useTitle names the browser tab after the page.
let workspaceName = "ws";
let waitingCount = 0;
let pageTitle = "";

function applyTitle() {
  const parts = [pageTitle, workspaceName].filter(Boolean);
  document.title = (waitingCount ? `(${waitingCount}) ` : "") + parts.join(" · ");
}

export function setTitleContext(name: string, waiting: number) {
  workspaceName = name || "ws";
  waitingCount = waiting;
  applyTitle();
}

// It names it only while on: a part that's drawn as a page or beside one
// (a plugin's frame) names the tab only as the page.
export function useTitle(title: string | undefined, on = true) {
  useEffect(() => {
    if (!on) return;
    pageTitle = title ?? "";
    applyTitle();
  }, [title, on]);
}
