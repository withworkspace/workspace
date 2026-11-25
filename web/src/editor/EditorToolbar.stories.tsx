import type { Meta, StoryObj } from "@storybook/react-vite";
import { EditorLinkBox } from "./EditorLinkBox";
import { EditorTool, EditorToolbar, EditorToolSep } from "./EditorToolbar";

// The editor's toolbar, alone: over a document, with what the selection's
// in lit; before there's an editor, nothing to press; in a card; in a
// page's bar; and showing the markdown as text. Then the box that asks for
// a link's address, for words and for a link already.

const meta: Meta = { title: "Editor/EditorToolbar" };
export default meta;

const none = () => {};

function Tools({ disabled = false }: { disabled?: boolean }) {
  return (
    <>
      <EditorTool icon="H1" title="Heading 1" on={false} disabled={disabled} onPress={none} />
      <EditorTool icon="H2" title="Heading 2" on={!disabled} disabled={disabled} onPress={none} />
      <EditorTool icon="H3" title="Heading 3" on={false} disabled={disabled} onPress={none} />
      <EditorToolSep />
      <EditorTool icon="bold" title="Bold" on={!disabled} disabled={disabled} onPress={none} />
      <EditorTool icon="italic" title="Italic" on={false} disabled={disabled} onPress={none} />
      <EditorTool icon="link" title="Link" on={false} disabled={disabled} onPress={none} />
      <EditorToolSep />
      <EditorTool icon="list" title="Bullets" on={false} disabled={disabled} onPress={none} />
      <EditorTool icon="code-block" title="Code block" on={false} disabled={disabled} onPress={none} />
    </>
  );
}

const frame = (label: string, children: React.ReactNode) => (
  <div style={{ display: "grid", gap: 6 }}>
    <span className="dim">{label}</span>
    {children}
  </div>
);

export const Every: StoryObj = {
  render: () => (
    <div style={{ padding: 32, maxWidth: 760, display: "grid", gap: 28 }}>
      {frame(
        "Over a document",
        <EditorToolbar onSource={none}>
          <Tools />
        </EditorToolbar>,
      )}
      {frame(
        "No editor yet",
        <EditorToolbar onSource={none}>
          <Tools disabled />
        </EditorToolbar>,
      )}
      {frame(
        "In a card",
        <EditorToolbar compact onSource={none}>
          <Tools />
        </EditorToolbar>,
      )}
      {frame(
        "In a page's bar",
        <EditorToolbar placed onSource={none}>
          <Tools />
        </EditorToolbar>,
      )}
      {frame(
        "Its markdown",
        <EditorToolbar source onSource={none}>
          <span className="dim">Markdown, as it's saved</span>
        </EditorToolbar>,
      )}
    </div>
  ),
};

export const LinkBox: StoryObj = {
  render: () => (
    <div style={{ padding: 32, display: "grid", gap: 80 }}>
      <div style={{ position: "relative", height: 40 }}>
        <EditorLinkBox href="" top={0} left={0} onDone={none} onClose={none} />
      </div>
      <div style={{ position: "relative", height: 40 }}>
        <EditorLinkBox href="https://example.com/limits" top={0} left={0} onDone={none} onClose={none} />
      </div>
    </div>
  ),
};
