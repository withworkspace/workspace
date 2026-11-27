import { baseKeymap, chainCommands, exitCode, lift, setBlockType, toggleMark, wrapIn } from "prosemirror-commands";
import { redo, undo } from "prosemirror-history";
import { InputRule, inputRules, textblockTypeInputRule, undoInputRule, wrappingInputRule } from "prosemirror-inputrules";
import { keymap } from "prosemirror-keymap";
import type { MarkType, Node as PMNode, NodeType } from "prosemirror-model";
import { liftListItem, sinkListItem, splitListItem, wrapInList } from "prosemirror-schema-list";
import { type Command, type EditorState, NodeSelection, type Plugin, TextSelection } from "prosemirror-state";
import { findWrapping } from "prosemirror-transform";
import { schema } from "./schema";

// What editing a document does: its keys, the markdown you can type
// (# for a heading, - for a list, **bold**), and the commands the
// toolbar runs. Markdown's own syntax is the shortcut, so whoever writes
// markdown already knows them.

const { nodes: N, marks: M } = schema;

// ListKind is a list as you'd name it: bullets, numbers or tasks.
export type ListKind = "bullet" | "ordered" | "task";

// listAt is the innermost list around the selection, and where it is.
function listAt(state: EditorState): { node: PMNode; pos: number; depth: number } | undefined {
  const { $from } = state.selection;
  for (let d = $from.depth; d > 0; d--) {
    const node = $from.node(d);
    if (node.type === N.bullet_list || node.type === N.ordered_list) return { node, pos: $from.before(d), depth: d };
  }
  return undefined;
}

function kindOf(list: PMNode): ListKind {
  if (list.type === N.ordered_list) return "ordered";
  let task = list.childCount > 0;
  list.forEach((item) => {
    if (item.attrs.checked === null) task = false;
  });
  return task ? "task" : "bullet";
}

export function listActive(state: EditorState, kind: ListKind): boolean {
  const list = listAt(state);
  return !!list && kindOf(list.node) === kind;
}

// toggleList makes the selection a list of a kind: out of it, if it's one
// already; into it from another kind; into one, if it isn't in a list.
export function toggleList(kind: ListKind): Command {
  return (state, dispatch) => {
    const type = kind === "ordered" ? N.ordered_list : N.bullet_list;
    const list = listAt(state);
    if (list && kindOf(list.node) === kind) return liftListItem(N.list_item)(state, dispatch);
    if (list) {
      if (dispatch) {
        const tr = state.tr.setNodeMarkup(list.pos, type, type === N.ordered_list ? { start: 1, spread: list.node.attrs.spread } : { spread: list.node.attrs.spread });
        list.node.forEach((item, offset) => {
          tr.setNodeMarkup(list.pos + 1 + offset, null, { ...item.attrs, checked: kind === "task" ? (item.attrs.checked ?? false) : null });
        });
        dispatch(tr);
      }
      return true;
    }
    return wrapInList(type)(state, (tr) => {
      if (kind === "task") {
        // The items just made: those in the list around the selection.
        const $from = tr.doc.resolve(tr.mapping.map(state.selection.from));
        for (let d = $from.depth; d > 0; d--) {
          if ($from.node(d).type !== N.bullet_list) continue;
          const pos = $from.before(d);
          $from.node(d).forEach((item, offset) => {
            tr.setNodeMarkup(pos + 1 + offset, null, { ...item.attrs, checked: false });
          });
          break;
        }
      }
      dispatch?.(tr);
    });
  };
}

export function headingLevel(state: EditorState): number {
  const parent = state.selection.$from.parent;
  return parent.type === N.heading ? parent.attrs.level : 0;
}

// toggleHeading makes the block a heading of that level, or a paragraph if
// it's one already.
export function toggleHeading(level: number): Command {
  return (state, dispatch) =>
    headingLevel(state) === level ? setBlockType(N.paragraph)(state, dispatch) : setBlockType(N.heading, { level })(state, dispatch);
}

export function inside(state: EditorState, type: NodeType): boolean {
  const { $from } = state.selection;
  for (let d = $from.depth; d > 0; d--) if ($from.node(d).type === type) return true;
  return false;
}

export const toggleQuote: Command = (state, dispatch) => (inside(state, N.blockquote) ? lift(state, dispatch) : wrapIn(N.blockquote)(state, dispatch));

export const toggleCode: Command = (state, dispatch) =>
  state.selection.$from.parent.type === N.code_block ? setBlockType(N.paragraph)(state, dispatch) : setBlockType(N.code_block)(state, dispatch);

// insertRule puts a horizontal rule by the line the cursor's on: before
// it, from its start; after it, from anywhere else; in its place, if it's
// empty (with a new line after, to go on writing).
export const insertRule: Command = (state, dispatch) => {
  const { $from } = state.selection;
  if (!$from.parent.isTextblock) return false;
  if (dispatch) {
    const tr = state.tr;
    if (!$from.parent.content.size) {
      tr.replaceWith($from.before(), $from.after(), [N.horizontal_rule.create(), N.paragraph.create()]);
      tr.setSelection(TextSelection.create(tr.doc, $from.before() + 2));
    } else if ($from.parentOffset === 0) tr.insert($from.before(), N.horizontal_rule.create());
    else tr.insert($from.after(), N.horizontal_rule.create());
    dispatch(tr.scrollIntoView());
  }
  return true;
};

export function markActive(state: EditorState, type: MarkType): boolean {
  const { from, to, empty, $from } = state.selection;
  if (empty) return !!type.isInSet(state.storedMarks ?? $from.marks());
  return state.doc.rangeHasMark(from, to, type);
}

// linkAt is the link the cursor is in (or the selection starts in): its
// address and extent.
export function linkAt(state: EditorState): { href: string; from: number; to: number } | undefined {
  const { $from } = state.selection;
  const mark = M.link.isInSet($from.marks()) ?? ($from.nodeAfter && M.link.isInSet($from.nodeAfter.marks));
  if (!mark) return undefined;
  const parent = $from.parent;
  const start = $from.start();
  let from = -1;
  let to = -1;
  parent.forEach((child, offset) => {
    const a = start + offset;
    const b = a + child.nodeSize;
    if (mark.isInSet(child.marks)) {
      if (from < 0 || a > to) {
        // A new run of the link: the one the cursor's in, if it's this.
        if (a <= $from.pos && $from.pos <= b) {
          from = a;
          to = b;
        } else if (from < 0 || to < $from.pos) {
          from = a;
          to = b;
        }
      } else {
        to = b;
      }
    }
  });
  return from < 0 ? undefined : { href: mark.attrs.href, from, to };
}

// setLink links from..to to href, or unlinks it if href is empty. Over
// nothing, it writes the address and links that.
export function setLink(from: number, to: number, href: string): Command {
  return (state, dispatch) => {
    if (!dispatch) return true;
    const tr = state.tr;
    if (!href) tr.removeMark(from, to, M.link);
    else if (from === to) {
      tr.insert(from, schema.text(href, [M.link.create({ href })]));
      to = from + href.length;
    } else tr.removeMark(from, to, M.link).addMark(from, to, M.link.create({ href }));
    dispatch(tr.setSelection(TextSelection.create(tr.doc, to)).removeStoredMark(M.link).scrollIntoView());
    return true;
  };
}

// --- Keys

// endOfParagraph is the paragraph the cursor's at the end of, if it is.
function endOfParagraph(state: EditorState) {
  const { $from, empty } = state.selection;
  const para = $from.parent;
  return empty && para.type === N.paragraph && $from.parentOffset === para.content.size ? $from : undefined;
}

// fence is Enter on a line that's just ``` (and a language): it's a code
// block.
export const fence: Command = (state, dispatch) => {
  const $from = endOfParagraph(state);
  const found = $from && /^```([\w+#.-]*)$/.exec($from.parent.textContent);
  if (!$from || !found) return false;
  if (dispatch) {
    const tr = state.tr.replaceWith($from.before(), $from.after(), N.code_block.create({ lang: found[1] }));
    dispatch(tr.setSelection(TextSelection.create(tr.doc, $from.before() + 1)).scrollIntoView());
  }
  return true;
};

// rule is Enter on a line that's just ---: a rule.
const rule: Command = (state, dispatch) => {
  const $from = endOfParagraph(state);
  if (!$from || !/^(-{3,}|\*{3,}|_{3,})$/.test($from.parent.textContent)) return false;
  if (dispatch) {
    const tr = state.tr.replaceWith($from.before(), $from.after(), [N.horizontal_rule.create(), N.paragraph.create()]);
    dispatch(tr.setSelection(TextSelection.create(tr.doc, $from.before() + 2)).scrollIntoView());
  }
  return true;
};

const fenceOrRule = chainCommands(fence, rule);

// splitItem is Enter in a list: a new item (a task, after a task), or out
// of the list from an empty one.
export const splitItem: Command = (state, dispatch) => {
  const { $from } = state.selection;
  if ($from.parent.type.spec.code || $from.depth < 2 || $from.node(-1).type !== N.list_item) return false;
  const task = $from.node(-1).attrs.checked !== null;
  return splitListItem(N.list_item, task ? { checked: false, spread: false } : undefined)(state, dispatch);
};

// unItem is Backspace at the start of an item: a task loses its box
// first, then the item leaves its list.
export const unItem: Command = (state, dispatch) => {
  const { $from, empty } = state.selection;
  if (!empty || $from.parentOffset !== 0 || $from.depth < 2 || $from.node(-1).type !== N.list_item || $from.index(-1) !== 0) return false;
  const item = $from.node(-1);
  if (item.attrs.checked !== null) {
    dispatch?.(state.tr.setNodeMarkup($from.before(-1), null, { ...item.attrs, checked: null }));
    return true;
  }
  return liftListItem(N.list_item)(state, dispatch);
};

const hardBreak: Command = (state, dispatch) => {
  dispatch?.(state.tr.replaceSelectionWith(N.hard_break.create()).scrollIntoView());
  return true;
};

// leaveCode is ↓ (or →) at the very end of a code block with nothing
// after it, or with one selected whole (a diagram, drawn): a paragraph
// after it to go on in, since there's nowhere else to go.
export const leaveCode: Command = (state, dispatch) => {
  const sel = state.selection;
  let after: number;
  if (sel instanceof NodeSelection) {
    if (!sel.node.type.spec.code) return false;
    after = sel.to;
  } else {
    const { $head, empty } = sel;
    if (!empty || !$head.parent.type.spec.code || $head.parentOffset !== $head.parent.content.size) return false;
    after = $head.after();
  }
  const $after = state.doc.resolve(after);
  if ($after.index() !== $after.parent.childCount) return false;
  if (dispatch) {
    const tr = state.tr.insert(after, N.paragraph.create());
    dispatch(tr.setSelection(TextSelection.create(tr.doc, after + 1)).scrollIntoView());
  }
  return true;
};

export const indentCode: Command = (state, dispatch) => {
  if (!state.selection.$from.parent.type.spec.code) return false;
  dispatch?.(state.tr.insertText("  ").scrollIntoView());
  return true;
};

// keys is the editor's keymap. submit and cancel are ⌘⏎ and Esc; link is
// ⌘K, which asks for an address.
export function keys(on: { submit?: () => void; cancel?: () => void; link: () => void }): Plugin[] {
  const call = (fn?: () => void): Command => () => {
    if (!fn) return false;
    fn();
    return true;
  };
  return [
    keymap({
      "Mod-z": undo,
      "Shift-Mod-z": redo,
      "Mod-y": redo,
      "Mod-b": toggleMark(M.strong),
      "Mod-i": toggleMark(M.em),
      "Mod-e": toggleMark(M.code),
      "Shift-Mod-x": toggleMark(M.strike),
      "Mod-k": call(on.link),
      "Mod-Alt-0": setBlockType(N.paragraph),
      "Mod-Alt-1": toggleHeading(1),
      "Mod-Alt-2": toggleHeading(2),
      "Mod-Alt-3": toggleHeading(3),
      "Shift-Mod-7": toggleList("ordered"),
      "Shift-Mod-8": toggleList("bullet"),
      "Shift-Mod-9": toggleList("task"),
      "Shift-Mod-.": toggleQuote,
      Enter: chainCommands(fenceOrRule, splitItem),
      "Shift-Enter": chainCommands(exitCode, hardBreak),
      Backspace: chainCommands(undoInputRule, unItem),
      ArrowDown: leaveCode,
      ArrowRight: leaveCode,
      Tab: chainCommands(sinkListItem(N.list_item), indentCode),
      "Shift-Tab": liftListItem(N.list_item),
      "Mod-Enter": call(on.submit),
      Escape: call(on.cancel),
    }),
    keymap(baseKeymap),
  ];
}

// --- Markdown as you type

// markRule is **text** (or *text*, `text`…) becoming text with a mark,
// as the closing marker's typed.
function markRule(re: RegExp, type: MarkType): InputRule {
  return new InputRule(re, (state, match, start, end) => {
    const words = match[1];
    const from = start + match[0].indexOf(words);
    const to = from + words.length;
    const tr = state.tr.delete(to, end).delete(start, from);
    return tr.addMark(start, start + words.length, type.create()).removeStoredMark(type);
  });
}

// A task: [ ] (or [x]) at the start of an item, or of a paragraph, which
// becomes one.
const taskRule = new InputRule(/^\[([ xX]?)\]\s$/, (state, match, start, end) => {
  const checked = match[1].toLowerCase() === "x";
  const $start = state.doc.resolve(start);
  const tr = state.tr.delete(start, end);
  if ($start.depth >= 2 && $start.node(-1).type === N.list_item && $start.index(-1) === 0) {
    return tr.setNodeMarkup($start.before(-1), null, { ...$start.node(-1).attrs, checked });
  }
  const range = tr.doc.resolve(start).blockRange();
  const wrapping = range && findWrapping(range, N.bullet_list);
  if (!range || !wrapping) return null;
  tr.wrap(range, wrapping);
  return tr.setNodeMarkup(range.start + 1, null, { checked, spread: false });
});

// A web address, as the space after it's typed, becomes a link to it.
const linkRule = new InputRule(/(?:^|[\s(])((?:https?:\/\/|www\.)[^\s<]*[^\s<.,:;"')\]!?])(\s)$/, (state, match, _start, end) => {
  const url = match[1];
  const from = end - url.length;
  if (M.link.isInSet(state.doc.resolve(from).marks())) return null;
  return state.tr.addMark(from, end, M.link.create({ href: url.startsWith("www.") ? "http://" + url : url })).insertText(match[2], end);
});

// ruleSets is the markdown you can type, by what it makes, for a box that
// offers some of it (RichText.tsx). The document has them all (rules).
export const ruleSets = {
  heading: [textblockTypeInputRule(/^(#{1,6})\s$/, N.heading, (m) => ({ level: m[1].length }))],
  quote: [wrappingInputRule(/^\s*>\s$/, N.blockquote)],
  task: [taskRule],
  list: [
    wrappingInputRule(/^\s*([-+*])\s$/, N.bullet_list),
    wrappingInputRule(
      /^(\d+)[.)]\s$/,
      N.ordered_list,
      (m) => ({ start: Number(m[1]) }),
      (m, node) => node.childCount + node.attrs.start === Number(m[1]),
    ),
  ],
  code: [textblockTypeInputRule(/^```([\w+#.-]*)\s$/, N.code_block, (m) => ({ lang: m[1] }))],
  marks: [
    markRule(/\*\*([^*\s](?:[^*]*[^*\s])?)\*\*$/, M.strong),
    markRule(/(?<![\w_])__([^_\s](?:[^_]*[^_\s])?)__$/, M.strong),
    markRule(/(?<![*\w])\*([^*\s](?:[^*]*[^*\s])?)\*$/, M.em),
    markRule(/(?<![_\w])_([^_\s](?:[^_]*[^_\s])?)_$/, M.em),
    markRule(/~~([^~\s](?:[^~]*[^~\s])?)~~$/, M.strike),
    markRule(/`([^`]+)`$/, M.code),
    linkRule,
  ],
};

const R = ruleSets;
export const rules: Plugin = inputRules({ rules: [...R.heading, ...R.quote, ...R.task, ...R.list, ...R.code, ...R.marks] });
