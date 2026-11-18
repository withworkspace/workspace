import type { Named, ThingBlock } from "./refs";

// A plugin's thing, drawn: its fields, as its plugin told ws of them, laid
// out as the plugin declared for where it's shown (a view: a list of
// blocks, each showing a field). What's worked out here is which blocks
// are drawn and what each says; ThingView draws them.

// Surface is where a thing is drawn: the card under its name, or the pane
// beside the list in Find.
export type Surface = "card" | "pane";

// ThingRecord is a thing's name and its fields by name.
export interface ThingRecord {
  key: string;
  fields: Record<string, string>;
}

// keyField is the field a block shows the thing's own name by.
export const keyField = "key";

const blocks = ["title", "subtitle", "badge", "field", "markdown"] as const;
type BlockKind = (typeof blocks)[number];

// Drawn is a block as it's drawn.
export interface Drawn {
  block: BlockKind;
  field: string;
  value: string;
  // label is what a field is called, before its value.
  label?: string;
  // time: its value is a time (RFC 3339), said as ws says times.
  time?: boolean;
  // lines is how many lines its markdown is kept to.
  lines?: number;
  // icon: it's the first title, which has the kind's icon.
  icon?: boolean;
  // inline: it's a badge on the line of the title, subtitle or badge
  // before it.
  inline?: boolean;
}

// typeOf is what a field is: one that isn't declared, or of a type ws
// doesn't know, is text.
function typeOf(named: Named | undefined, field: string): string {
  const type = named?.fields?.find((f) => f.name === field)?.type;
  return type === "title" || type === "state" || type === "time" || type === "markdown" ? type : "text";
}

// viewOf is the blocks of a kind's view for a surface: as its plugin
// declared it, or, where it declared none, one made from its fields. The
// card is its title, its name and where it stands; the pane is those, then
// each of its text and time fields with its label, then its markdown. A
// kind with no fields has nothing to draw.
export function viewOf(named: Named | undefined, surface: Surface): ThingBlock[] {
  const declared = named?.views?.[surface];
  if (declared?.length) return declared;
  const fields = named?.fields ?? [];
  if (!fields.length) return [];
  const of = (type: string) => fields.filter((f) => typeOf(named, f.name) === type);
  const view: ThingBlock[] = [...of("title").slice(0, 1).map((f) => ({ block: "title", field: f.name })), { block: "subtitle", field: keyField }, ...of("state").slice(0, 1).map((f) => ({ block: "badge", field: f.name }))];
  if (surface === "card") return view;
  for (const f of fields) if (["text", "time"].includes(typeOf(named, f.name))) view.push({ block: "field", field: f.name });
  for (const f of of("markdown")) view.push({ block: "markdown", field: f.name });
  return view;
}

// thingBlocks is what's drawn of a thing on a surface, in order: each
// block of its kind's view that ws knows and that has something to show.
// A block shows its field's value, or the thing's name for key.
export function thingBlocks(named: Named | undefined, record: ThingRecord, surface: Surface): Drawn[] {
  const out: Drawn[] = [];
  let titled = false;
  for (const b of viewOf(named, surface)) {
    const block = blocks.find((k) => k === b.block);
    if (!block) continue; // one ws doesn't know
    const value = (b.field === keyField ? record.key : record.fields[b.field])?.trim();
    if (!value) continue;
    const d: Drawn = { block, field: b.field, value };
    const type = typeOf(named, b.field);
    if (block !== "markdown" && b.field !== keyField && type === "time") d.time = true;
    switch (block) {
      case "title":
        if (!titled) d.icon = titled = true;
        break;
      case "badge": {
        const before = out[out.length - 1]?.block;
        if (before === "title" || before === "subtitle" || before === "badge") d.inline = true;
        break;
      }
      case "field":
        d.label = b.label || named?.fields?.find((f) => f.name === b.field)?.label || b.field;
        break;
      case "markdown":
        if (b.lines && b.lines > 0) d.lines = Math.floor(b.lines);
        break;
    }
    out.push(d);
  }
  return out;
}
