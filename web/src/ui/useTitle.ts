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

export function useTitle(title: string | undefined) {
  useEffect(() => {
    pageTitle = title ?? "";
    applyTitle();
  }, [title]);
}
