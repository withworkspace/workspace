import type { Node as PMNode } from "prosemirror-model";
import type { CellSource, PageCell } from "../api";
import type { CellState } from "../editor/cells";
import { cellState } from "./cell";

// The map: the page as what it's made of and what it links to.
export interface MapData {
  outline: { pos: number; level: number; text: string }[];
  cells: { pos: number; name: string; prompt: string; state: CellState }[];
  // open: what's not ticked off: questions a check raised, tasks you wrote.
  open: { pos: number; text: string }[];
  // sources: the files its cells were written from, with how many cells
  // rest on each.
  sources: { src: CellSource; cells: number }[];
}

export function mapOf(doc: PMNode, cells: PageCell[], root: string): MapData {
  const map: MapData = { outline: [], cells: [], open: [], sources: [] };
  const at = new Map<string, number>();
  doc.forEach((top, offset) => {
    if (top.type.name === "heading") map.outline.push({ pos: offset, level: top.attrs.level, text: top.textContent });
    if (top.type.name === "cell") {
      const cell = cells.find((c) => c.id === top.attrs.id);
      map.cells.push({ pos: offset, name: top.attrs.id || top.attrs.key, prompt: top.attrs.prompt, state: cell ? cellState(cell, root) : { status: "starting" } });
      for (const src of cell?.sources ?? []) {
        const i = at.get(src.path);
        if (i === undefined) at.set(src.path, map.sources.push({ src, cells: 1 }) - 1);
        else map.sources[i].cells++;
      }
    }
    top.descendants((node, pos) => {
      if (node.type.name === "list_item" && node.attrs.checked === false && node.firstChild?.textContent) {
        map.open.push({ pos: offset + 1 + pos, text: node.firstChild.textContent });
      }
      return true;
    });
  });
  return map;
}
