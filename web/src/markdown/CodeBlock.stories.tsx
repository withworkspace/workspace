import type { Meta, StoryObj } from "@storybook/react-vite";
import { highlightCode, useHighlighter } from "../review/highlight";
import { CodeBlock } from "./CodeBlock";
import { Mermaid } from "./Mermaid";

// A block of code, alone: with its language, highlighted; without one;
// the code as it is. And a diagram: drawn, and one that can't be.

const meta: Meta = { title: "Markdown/CodeBlock" };
export default meta;

const go = `func limit(key string) int {
	return 100
}`;

function Blocks() {
  useHighlighter();
  return (
    <div style={{ padding: 32, maxWidth: 640, display: "grid", gap: 16 }}>
      <CodeBlock lang="go" html={highlightCode(go, "go")} />
      <CodeBlock html={highlightCode("no language", undefined)} />
      <CodeBlock>{"$ ws mission show\nas it is, not highlighted"}</CodeBlock>
    </div>
  );
}

export const Every: StoryObj = { render: () => <Blocks /> };

export const Diagrams: StoryObj = {
  render: () => (
    <div style={{ padding: 32, maxWidth: 640, display: "grid", gap: 16 }}>
      <Mermaid code={"graph LR\n  A[Asked] --> B[Limited]"} />
      <Mermaid code={"graph LR\n  A[ -->"} />
    </div>
  ),
};
