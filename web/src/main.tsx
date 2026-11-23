// The global styles first: a component's CSS module comes after them, so
// its rules win over a global one of the same weight (.row, then the
// module's .row beside it). Keep this import above App's.
import "./styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { App } from "./App";
import { ContextMenuProvider } from "./ui/ContextMenu";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ContextMenuProvider>
        <App />
      </ContextMenuProvider>
    </BrowserRouter>
  </StrictMode>,
);
