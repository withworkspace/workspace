import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { CodeNavCard } from "./CodeNavCard";
import { CodeNavPanel } from "./CodeNavPanel";
import { CodeNavPicker } from "./CodeNavPicker";
import { CodeNavUses } from "./CodeNavUses";

// Following names in a review, from its parts: a name's card (one
// definition, several it may be, still looking), d's list of a line's
// names, and the panel beside the diff with who uses a name.

const meta: Meta = { title: "Review/CodeNav" };
export default meta;

const none = () => {};
const at = (top: number, left = 40) => new DOMRect(left, top, 80, 18);
const target = { path: "internal/nav/nav.go", line: 12, col: 6, endLine: 12, kind: "func", signature: "func Names(src string) []Name", doc: "Names are the names a file declares, in the order it declares them." };

export const Cards: StoryObj = {
  render: () => (
    <div style={{ height: 520 }}>
      <CodeNavCard answer={{ name: "Names", answer: "resolved", targets: [target] }} rect={at(20)} pinned={false} onClose={none} onGo={none} onFollow={none} />
      <CodeNavCard
        answer={{ name: "Read", answer: "candidates", how: "by name", targets: [target, { ...target, line: 40, signature: "func (f *File) Read() []byte" }], partial: true }}
        rect={at(160)}
        pinned
        onClose={none}
        onGo={none}
        onFollow={none}
      />
      <CodeNavCard answer={{ name: "strings.Cut", answer: "external", targets: [], external: { module: "strings", name: "Cut", url: "https://pkg.go.dev/strings#Cut" } }} rect={at(320)} pinned={false} onClose={none} onGo={none} onFollow={none} />
      <CodeNavCard rect={at(420)} pinned={false} onClose={none} onGo={none} onFollow={none} />
    </div>
  ),
};

export const Picker: StoryObj = {
  render: () => (
    <div style={{ height: 240 }}>
      <CodeNavPicker
        names={[
          { from: 1, text: "Names", part: "d" },
          { from: 8, text: "strings", part: "i" },
          { from: 16, text: "Cut", part: "m" },
        ]}
        rect={at(20)}
        onClose={none}
        onPick={none}
      />
    </div>
  ),
};

function Uses() {
  const [pick, setPick] = useState(0);
  const [open, setOpen] = useState(false);
  const use = (i: number, path: string, line: number, text: string, changed = false) => ({ key: `${i}`, path, line, text, changed, picked: pick === i, onEnter: () => setPick(i), onGo: none });
  return (
    <div style={{ height: 520, position: "relative" }}>
      <CodeNavPanel label="Who uses Names" title={<span>Who uses <code>Names</code></span>} said="3 uses, maybe more · with the change" uses onBack={none} onClose={none}>
        <CodeNavUses
          def={{ signature: target.signature, path: target.path, line: 12 }}
          groups={[
            { title: "In this change", named: true, uses: [use(0, "internal/nav/outline.go", 30, "for _, n := range Names(src) {", true)], maybes: [], open: false },
            { title: "Elsewhere", named: true, uses: [use(1, "internal/web/nav.go", 88, "names := nav.Names(string(b))")], maybes: [use(2, "internal/doc/toc.go", 14, "Names(text)")], open },
          ]}
          onMaybes={() => setOpen((o) => !o)}
        />
      </CodeNavPanel>
    </div>
  );
}

export const WhoUsesIt: StoryObj = { render: () => <Uses /> };

export const AFileBeingRead: StoryObj = {
  render: () => (
    <CodeNavPanel label="nav.go" title={<span className="mono">internal/nav/nav.go:12</span>} said="with the change · 88cf294" how="by name" note="Reading internal/nav/nav.go…" onBack={none} onClose={none}>
      {null}
    </CodeNavPanel>
  ),
};
