import { baseKeymap, chainCommands, newlineInCode, toggleMark } from "prosemirror-commands";
import { redo, undo } from "prosemirror-history";
import { inputRules, undoInputRule } from "prosemirror-inputrules";
import { keymap } from "prosemirror-keymap";
import { Fragment, type Mark, type Node as PMNode, Slice } from "prosemirror-model";
import { liftListItem, sinkListItem } from "prosemirror-schema-list";
import { type Command, Plugin, TextSelection } from "prosemirror-state";
import { Decoration, DecorationSet } from "prosemirror-view";
import { type SendKey, sent } from "../ui/sendKey";
import { fence, indentCode, leaveCode, ruleSets, splitItem, toggleList, unItem } from "./commands";
import { schema } from "./schema";

// What a box that's written and sent (RichText.tsx) does with its words:
// the markdown it offers (Features), its keys, and what's pasted into it.
// Its document is the document editor's (schema.ts), so its markdown is
// read and written as a document's is (markdown.ts); what it doesn't
// offer can't be typed, or pasted, into it.

const { nodes: N, marks: M } = schema;

// Features is the markdown a box offers. With none it's words, in
// paragraphs: marks are bold, italic, struck, code and links; lists are
// bullets and numbers; quote is a quote; code is a block of code.
export interface Features {
  marks?: boolean;
  lists?: boolean;
  quote?: boolean;
  code?: boolean;
}

// --- What's typed

export function composeRules(f: Features): Plugin {
  return inputRules({
    rules: [...(f.quote ? ruleSets.quote : []), ...(f.lists ? ruleSets.list : []), ...(f.code ? ruleSets.code : []), ...(f.marks ? ruleSets.marks : [])],
  });
}

// --- Keys

// codeEnter is Enter in a block of code: a new line in it, or, on an empty
// last line, out of it, into a paragraph after (the empty line taken off).
const codeEnter: Command = (state, dispatch) => {
  const { $head, empty } = state.selection;
  if (!$head.parent.type.spec.code) return false;
  const text = $head.parent.textContent;
  if (!empty || $head.parentOffset !== text.length || !text.endsWith("\n")) return newlineInCode(state, dispatch);
  if (dispatch) {
    const tr = state.tr.delete($head.pos - 1, $head.pos);
    const at = tr.mapping.map($head.after());
    tr.insert(at, N.paragraph.create());
    dispatch(tr.setSelection(TextSelection.create(tr.doc, at + 1)).scrollIntoView());
  }
  return true;
};

// newLine is Enter as a document has it: a block of code, or a list's
// next item (out of the list from an empty one), or a new paragraph.
const newLine: Command = chainCommands(fence, codeEnter, splitItem, baseKeymap.Enter);

// itemOut is Enter in an empty item: out of its list (a list's own Enter
// leaves that to a document's, which a box that sends hasn't).
const itemOut: Command = (state, dispatch) => {
  const { $from, empty } = state.selection;
  if (!empty || $from.parent.content.size || $from.depth < 2 || $from.node(-1).type !== N.list_item) return false;
  return liftListItem(N.list_item)(state, dispatch);
};

// goesOn is the Enter that sends still going on with what it's in: a
// line in a block of code, an item of a list, a line of ``` that's a
// block of code; out of the list from an empty item. Elsewhere, it sends.
const goesOn: Command = chainCommands(fence, codeEnter, splitItem, itemOut);

export interface KeysOn {
  // The key that sends, as it is when pressed (it's yours to change).
  send: () => SendKey;
  onSend?: (how: { other: boolean }) => void;
  onEscape?: () => void;
}

// enter is Enter, with shift and mod held or not: it sends as you've
// chosen (ui/sendKey.ts), and with ⏎ it goes on in a list or a block of
// code first; what isn't sending is a new line, as in a document.
export function enter(on: KeysOn, shift: boolean, mod: boolean): Command {
  return (state, dispatch) => {
    const how = on.onSend && sent({ key: "Enter", shiftKey: shift, metaKey: mod, ctrlKey: false }, on.send());
    if (!how) return newLine(state, dispatch);
    if (!mod && goesOn(state, dispatch)) return true;
    // Only asked whether it can: it can, and it's not sent.
    if (dispatch) on.onSend?.(how);
    return true;
  };
}

// composeKeys is a box's keys: Enter (enter), undo, the marks and lists it
// offers, Tab in a list or a block of code, and Escape if it hears it.
export function composeKeys(f: Features, on: KeysOn): Plugin[] {
  const key = (shift: boolean, mod: boolean) => enter(on, shift, mod);
  const bound: Record<string, Command> = {
    "Mod-z": undo,
    "Shift-Mod-z": redo,
    "Mod-y": redo,
    Enter: key(false, false),
    "Shift-Enter": key(true, false),
    "Mod-Enter": key(false, true),
    "Shift-Mod-Enter": key(true, true),
    "Ctrl-Enter": key(false, true),
    "Shift-Ctrl-Enter": key(true, true),
    Backspace: chainCommands(undoInputRule, unItem),
    ArrowDown: leaveCode,
    ArrowRight: leaveCode,
    Tab: chainCommands(sinkListItem(N.list_item), indentCode),
    "Shift-Tab": liftListItem(N.list_item),
  };
  if (on.onEscape) {
    const escape = on.onEscape;
    bound.Escape = () => {
      escape();
      return true;
    };
  }
  if (f.marks) {
    bound["Mod-b"] = toggleMark(M.strong);
    bound["Mod-i"] = toggleMark(M.em);
    bound["Mod-e"] = toggleMark(M.code);
    bound["Shift-Mod-x"] = toggleMark(M.strike);
  }
  if (f.lists) {
    bound["Shift-Mod-7"] = toggleList("ordered");
    bound["Shift-Mod-8"] = toggleList("bullet");
  }
  return [keymap(bound), keymap(baseKeymap)];
}

// --- What's pasted

// fit is what's pasted made of what the box offers: a heading (or what's
// on) its words in a paragraph, a list's items or a quote's paragraphs as
// paragraphs, a block of code its lines, a table a paragraph a row, a
// rule nothing, a picture its words; marks it doesn't offer are taken off.
// A document pasted into a conversation, say, keeps its lists and its
// code.
export function fit(slice: Slice, f: Features): Slice {
  const blocks = (content: Fragment): PMNode[] => {
    const out: PMNode[] = [];
    content.forEach((n) => out.push(...block(n)));
    return out;
  };
  const inline = (content: Fragment): PMNode[] => {
    const out: PMNode[] = [];
    content.forEach((n) => {
      if (n.type === N.image) {
        if (n.attrs.alt) out.push(schema.text(n.attrs.alt, marks(n.marks)));
      } else if (n.isText) out.push(n.mark(marks(n.marks)));
      else out.push(n);
    });
    return out;
  };
  const marks = (ms: readonly Mark[]) => (f.marks ? ms : []);
  const para = (kids: PMNode[]) => N.paragraph.create(null, kids);
  const block = (n: PMNode): PMNode[] => {
    switch (n.type) {
      case N.paragraph:
        return [para(inline(n.content))];
      case N.heading:
      case N.table_header:
      case N.table_cell:
        return [para(inline(n.content))];
      case N.bullet_list:
      case N.ordered_list:
        return f.lists ? [n.type.create(n.attrs, n.content.content.map((item) => item.type.create({ ...item.attrs, checked: null }, blocks(item.content))))] : blocks(n.content);
      case N.list_item:
        return f.lists ? [N.bullet_list.create(null, N.list_item.create({ ...n.attrs, checked: null }, blocks(n.content)))] : blocks(n.content);
      case N.blockquote:
        return f.quote ? [n.type.create(n.attrs, blocks(n.content))] : blocks(n.content);
      case N.code_block:
        return f.code ? [n] : n.textContent ? [para([schema.text(n.textContent)])] : [];
      case N.table: {
        const rows: PMNode[] = [];
        n.forEach((row) => {
          const cells: string[] = [];
          row.forEach((c) => cells.push(c.textContent));
          rows.push(para(cells.join(" · ") ? [schema.text(cells.join(" · "))] : []));
        });
        return rows;
      }
      case N.table_row: {
        const cells: string[] = [];
        n.forEach((c) => cells.push(c.textContent));
        return [para(cells.join(" · ") ? [schema.text(cells.join(" · "))] : [])];
      }
      case N.horizontal_rule:
        return [];
      case N.raw_block:
        return [para([schema.text(n.attrs.text)])];
      default:
        // A page's cell, or anything else that holds blocks: what's in it.
        return n.isTextblock ? [para(inline(n.content))] : blocks(n.content);
    }
  };
  // Words alone (a run of them, from a paragraph) are pasted as they are.
  let words = true;
  slice.content.forEach((n) => {
    if (!n.isInline) words = false;
  });
  if (words) return new Slice(Fragment.fromArray(inline(slice.content)), 0, 0);
  const out = blocks(slice.content);
  if (!out.length) return Slice.empty;
  const open = (n?: PMNode) => (n?.type === N.paragraph ? 1 : 0);
  return new Slice(Fragment.fromArray(out), Math.min(slice.openStart, open(out[0])), Math.min(slice.openEnd, open(out[out.length - 1])));
}

// --- What's handed back

// tidy is markdown as it's handed back: a space at the start or end of a
// line as it was typed, not as markdown keeps one (&#x20;), as while
// you're between words.
export function tidy(md: string): string {
  return md.replace(/(?<!\\)&#x20;(?=\n|$)/g, " ").replace(/(^|\n)&#x20;/g, "$1 ");
}

// --- What's drawn

// placeholder says what to write while there's nothing: on the first
// paragraph, as cls's ::before (the box's module draws it).
export function placeholder(text: () => string, cls: string): Plugin {
  return new Plugin({
    props: {
      decorations(state) {
        const said = text();
        const first = state.doc.firstChild;
        if (!said || state.doc.childCount !== 1 || !first?.isTextblock || first.content.size) return null;
        return DecorationSet.create(state.doc, [Decoration.node(0, first.nodeSize, { class: cls, "data-placeholder": said })]);
      },
    },
  });
}
