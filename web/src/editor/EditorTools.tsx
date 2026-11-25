import { toggleMark } from "prosemirror-commands";
import type { Command } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";
import { EditorTool, EditorToolSep } from "./EditorToolbar";
import { headingLevel, inside, insertRule, listActive, markActive, toggleCode, toggleHeading, toggleList, toggleQuote } from "./commands";
import { schema } from "./schema";

const { nodes: N, marks: M } = schema;
const mac = /Mac|iPhone|iPad/.test(navigator.platform);
const mod = mac ? "⌘" : "Ctrl+";
const alt = mac ? "⌥" : "Alt+";
const shift = mac ? "⇧" : "Shift+";

// EditorTools are the toolbar's buttons for an editor: what each does to
// it, and whether it's lit, from where its selection is. Until there's an
// editor, they're there and can't be pressed.
export function EditorTools({ view, onLink }: { view?: EditorView; onLink: () => void }) {
  const state = view?.state;
  const run = (cmd: Command) => () => {
    if (!view) return;
    cmd(view.state, view.dispatch, view);
    view.focus();
  };
  const level = state ? headingLevel(state) : 0;
  const tool = (key: string, icon: string, title: string, on: boolean, onPress: () => void) => <EditorTool key={key} icon={icon} title={title} on={on} disabled={!view} onPress={onPress} />;
  return (
    <>
      {[1, 2, 3].map((n) => tool(`h${n}`, `H${n}`, `Heading ${n} (${mod}${alt}${n})`, level === n, run(toggleHeading(n))))}
      <EditorToolSep />
      {tool("bold", "bold", `Bold (${mod}B)`, !!state && markActive(state, M.strong), run(toggleMark(M.strong)))}
      {tool("italic", "italic", `Italic (${mod}I)`, !!state && markActive(state, M.em), run(toggleMark(M.em)))}
      {tool("strike", "strike", `Strikethrough (${mod}${shift}X)`, !!state && markActive(state, M.strike), run(toggleMark(M.strike)))}
      {tool("code", "code", `Code (${mod}E)`, !!state && markActive(state, M.code), run(toggleMark(M.code)))}
      {tool("link", "link", `Link (${mod}K)`, !!state && markActive(state, M.link), onLink)}
      <EditorToolSep />
      {tool("bullet", "list", `Bullets (${mod}${shift}8)`, !!state && listActive(state, "bullet"), run(toggleList("bullet")))}
      {tool("ordered", "list-ordered", `Numbers (${mod}${shift}7)`, !!state && listActive(state, "ordered"), run(toggleList("ordered")))}
      {tool("task", "list-checks", `Tasks (${mod}${shift}9)`, !!state && listActive(state, "task"), run(toggleList("task")))}
      {tool("quote", "quote", `Quote (${mod}${shift}.)`, !!state && inside(state, N.blockquote), run(toggleQuote))}
      {tool("codeblock", "code-block", "Code block (```)", !!state && state.selection.$from.parent.type === N.code_block, run(toggleCode))}
      {tool("rule", "minus", "Rule (---)", false, run(insertRule))}
    </>
  );
}
