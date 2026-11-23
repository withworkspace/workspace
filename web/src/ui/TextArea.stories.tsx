import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef, useState } from "react";
import { useAutosize } from "./kit";
import { TextArea } from "./TextArea";

// Text areas: empty, with words, growing with what's typed (useAutosize),
// in mono, disabled, and one a page lets be dragged taller.

const meta: Meta = { title: "Components/TextArea" };
export default meta;

function One(props: React.ComponentProps<typeof TextArea> & { start?: string }) {
  const { start = "", ...rest } = props;
  const [value, setValue] = useState(start);
  return <TextArea value={value} onChange={(e) => setValue(e.target.value)} {...rest} />;
}

function Grows() {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [value, setValue] = useState("Type more lines here, and it grows to hold them.");
  useAutosize(ref, value);
  return <TextArea ref={ref} rows={1} value={value} onChange={(e) => setValue(e.target.value)} aria-label="Reply" />;
}

const row = { display: "grid", gridTemplateColumns: "100px 360px", gap: 12, alignItems: "start" } as const;

export const Boxes: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14 }}>
      <label style={row}>
        <span className="dim">Empty</span>
        <One rows={3} placeholder="What it came to: the finding, what landed, why it stops" aria-label="What it came to" />
      </label>
      <label style={row}>
        <span className="dim">With words</span>
        <One rows={3} start={"Rounding moved to the end, so a penny doesn't go missing.\nINV-0042 comes to £1,094.40 again."} aria-label="Summary" />
      </label>
      <label style={row}>
        <span className="dim">Grows</span>
        <Grows />
      </label>
      <label style={row}>
        <span className="dim">Mono</span>
        <One rows={2} className="mono" start={"go test ./...\npnpm test"} aria-label="Checks" />
      </label>
      <label style={row}>
        <span className="dim">Disabled</span>
        <One rows={2} start="It can't be changed now." disabled aria-label="Why" />
      </label>
      <label style={row}>
        <span className="dim">Dragged taller</span>
        <One rows={2} placeholder="Why it's discarded" style={{ resize: "vertical" }} aria-label="Why" />
      </label>
    </div>
  ),
};
