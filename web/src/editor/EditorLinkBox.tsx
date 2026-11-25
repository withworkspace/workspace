import { useState } from "react";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import css from "./EditorLinkBox.module.css";

type EditorLinkBoxProps = {
  // The address it has, if it's a link already.
  href: string;
  // Where it's drawn, in the document's box: under the words.
  top: number;
  left: number;
  // done: the address to link to ("" takes the link away), or nothing
  // for leaving it as it was.
  onDone: (to?: string) => void;
  // It's left without a word (the focus gone elsewhere).
  onClose: () => void;
};

// EditorLinkBox asks for a link's address, under the words it's on: ⏎
// links them, Escape leaves them, and a link's address can be changed or
// taken away.
export function EditorLinkBox({ href: had, top, left, onDone, onClose }: EditorLinkBoxProps) {
  const [href, setHref] = useState(had);
  return (
    <div className={css.box} style={{ top, left }}>
      <Input
        autoFocus
        value={href}
        placeholder="https://…"
        aria-label="Link address"
        onChange={(e) => setHref(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            onDone(href);
          }
          if (e.key === "Escape") {
            e.preventDefault();
            onDone();
          }
        }}
        onBlur={() => onClose()}
      />
      <Button variant="primary" size="small" onMouseDown={(e) => e.preventDefault()} onClick={() => onDone(href)}>
        {had ? "Change" : "Link"}
      </Button>
      {had && (
        <Button variant="ghost" size="small" onMouseDown={(e) => e.preventDefault()} onClick={() => onDone("")}>
          Remove
        </Button>
      )}
    </div>
  );
}
