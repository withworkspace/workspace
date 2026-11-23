import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Input } from "./Input";
import { Modal } from "./Modal";

// A modal with a look of its own (a Dialog is the one with a head): a
// finder, here. Press the button; Escape or a press outside closes it,
// and the focus comes back to the button. The second hears Escape
// itself: the first press empties what's typed, the next closes it.

const meta: Meta = { title: "Components/Modal" };
export default meta;

const box = { width: "min(520px, 100%)", background: "var(--raised)", border: "1px solid var(--line)", borderRadius: 12, boxShadow: "var(--shadow-lg)", overflow: "hidden" } as const;

function Finder({ own }: { own?: boolean }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const close = () => {
    setOpen(false);
    setQ("");
  };
  return (
    <>
      <Button onClick={() => setOpen(true)}>{own ? "A finder that hears Escape itself" : "A finder"}</Button>
      {open && (
        <Modal onClose={close} escape={!own} aria-label="Jump to a file">
          <div style={box}>
            <Input
              autoFocus
              value={q}
              placeholder="Jump to a file"
              aria-label="Jump to a file"
              onChange={(e) => setQ(e.target.value)}
              onKeyDown={(e) => {
                if (!own || e.key !== "Escape") return;
                if (q) setQ("");
                else close();
              }}
              style={{ width: "100%", border: 0, borderRadius: 0, padding: 14, boxShadow: "none" }}
            />
            <p className="dim" style={{ margin: 0, padding: "10px 14px", borderTop: "1px solid var(--line)" }}>
              {q ? `Nothing matches “${q}”.` : "The page behind can't be reached while this is open."}
            </p>
          </div>
        </Modal>
      )}
    </>
  );
}

export const Finders: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "flex", gap: 8 }}>
      <Finder />
      <Finder own />
    </div>
  ),
};
