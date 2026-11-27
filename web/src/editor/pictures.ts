import { Plugin, PluginKey } from "prosemirror-state";
import { Decoration, DecorationSet, type EditorView } from "prosemirror-view";
import type { Pasted } from "../api";
import { isPicture } from "../pictures";
import { schema } from "./schema";

// Pictures pasted or dropped into a box that offers them (compose.ts's
// Features): each is kept by ws (POST /api/images) and put where it was
// pasted, named by its path, as markdown's ![image](<path>). While it's
// being kept, a placeholder holds its place, which moves as the words
// round it change; a picture that can't be kept leaves nothing, and says
// why.

const key = new PluginKey<DecorationSet>("pictures");

type Place = { add: { id: object; pos: number; cls: string } } | { remove: { id: object } };

// picturesPlugin keeps the placeholders: cls draws one.
export function picturesPlugin(): Plugin<DecorationSet> {
  return new Plugin<DecorationSet>({
    key,
    state: {
      init: () => DecorationSet.empty,
      apply(tr, set) {
        set = set.map(tr.mapping, tr.doc);
        const said = tr.getMeta(key) as Place | undefined;
        if (said && "add" in said) {
          const el = document.createElement("span");
          el.className = said.add.cls;
          el.setAttribute("aria-label", "Pasting picture");
          set = set.add(tr.doc, [Decoration.widget(said.add.pos, el, { id: said.add.id })]);
        } else if (said && "remove" in said) {
          set = set.remove(set.find(undefined, undefined, (spec) => spec.id === said.remove.id));
        }
        return set;
      },
    },
    props: {
      decorations: (state) => key.getState(state),
    },
  });
}

// picturesIn are the files pasted or dropped that are pictures.
export function picturesIn(data: DataTransfer | null): File[] {
  return data ? Array.from(data.files).filter(isPicture) : [];
}

// putPictures keeps each picture and puts it at pos (the selection, if
// none), each after the last, a placeholder there until it's kept.
export function putPictures(view: EditorView, files: File[], keep: (f: File) => Promise<Pasted>, cls: string, failed: (why: string) => void, pos?: number) {
  const tr = view.state.tr;
  if (pos == null) {
    if (!tr.selection.empty) tr.deleteSelection();
    pos = tr.selection.from;
  }
  const ids = files.map(() => ({}));
  // One placeholder a transaction: a transaction has one meta for a key.
  view.dispatch(tr.setMeta(key, { add: { id: ids[0], pos, cls } } satisfies Place));
  for (const id of ids.slice(1)) view.dispatch(view.state.tr.setMeta(key, { add: { id, pos, cls } } satisfies Place));
  files.forEach((file, i) => {
    const id = ids[i];
    const placed = () => {
      const set = key.getState(view.state);
      const found = set?.find(undefined, undefined, (spec) => spec.id === id);
      return found?.length ? found[0].from : undefined;
    };
    keep(file).then(
      (p) => {
        if (view.isDestroyed) return;
        const at = placed();
        if (at == null) return; // its words were taken away meanwhile
        const img = schema.nodes.image.create({ src: p.path, alt: "image" });
        view.dispatch(view.state.tr.insert(at, img).setMeta(key, { remove: { id } } satisfies Place));
      },
      (err: unknown) => {
        if (!view.isDestroyed) view.dispatch(view.state.tr.setMeta(key, { remove: { id } } satisfies Place));
        failed(err instanceof Error ? err.message : String(err));
      },
    );
  });
}
