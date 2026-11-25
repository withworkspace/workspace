import type { Meta, StoryObj } from "@storybook/react-vite";
import { useEffect, useState } from "react";
import { Button } from "./Button";
import { FileIcon } from "./FileIcon";
import { Finder, FinderNone, FinderRow } from "./Finder";
import { Text } from "./Text";

// A finder: press the button, then type. The cursor starts on the first
// row; the arrows (or Ctrl+N and Ctrl+P) and the pointer move it, and ⏎
// or a press picks. The second is still looking for a moment after each
// letter, and has a note under its list.

const meta: Meta = { title: "Components/Finder" };
export default meta;

const files = ["internal/invoice/invoice.go", "internal/invoice/money.go", "internal/invoice/tax.go", "internal/pdf/render.go", "internal/pdf/rows.go", "cmd/api/main.go", "README.md", "web/src/invoices/InvoiceList.tsx", "web/src/invoices/money.ts", "web/src/api.ts"];

function Files({ slow }: { slow?: boolean }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [picked, setPicked] = useState<string>();
  const [looking, setLooking] = useState(false);
  useEffect(() => {
    if (!slow) return;
    setLooking(true);
    const t = setTimeout(() => setLooking(false), 700);
    return () => clearTimeout(t);
  }, [q, slow]);
  const found = files.filter((f) => f.includes(q));
  return (
    <>
      <Button onClick={() => setOpen(true)}>{slow ? "Search the files" : "Find a file"}</Button>
      <Text dim>{picked ? `Picked ${picked}.` : "Nothing picked yet."}</Text>
      {open && (
        <Finder
          aria-label={slow ? "Search the files" : "Find a file"}
          placeholder={slow ? "Text in the files…" : "Find a file by a few of its letters…"}
          value={q}
          onChange={setQ}
          count={found.length}
          busy={looking}
          onPick={(i) => {
            setPicked(found[i]);
            setOpen(false);
          }}
          onClose={() => setOpen(false)}
          after={slow && <Text as="p" dim style={{ margin: 0, padding: "7px 14px", borderTop: "1px solid var(--line)" }}>{found.length} files</Text>}
        >
          {found.map((f, i) => (
            <FinderRow key={f} index={i}>
              <FileIcon path={f} />
              <Text mono>{f}</Text>
            </FinderRow>
          ))}
          {!found.length && <FinderNone>No file matches “{q}”.</FinderNone>}
        </Finder>
      )}
    </>
  );
}

export const Finders: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, gridTemplateColumns: "auto 1fr", alignItems: "center", justifyContent: "start" }}>
      <Files />
      <Files slow />
    </div>
  ),
};
