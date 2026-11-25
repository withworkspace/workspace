import type { PagePass } from "../api";
import type { CellState } from "../editor/cells";
import { Chip } from "../ui/Chip";
import { Icon, type IconName } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { clip } from "../ui/text";
import { sourceHref, sourceName } from "./cell";
import type { MapData } from "./map";
import { PageMap } from "./PageMap";
import { PageMapEmpty } from "./PageMapEmpty";
import { PageMapItem } from "./PageMapItem";
import { PageMapSection } from "./PageMapSection";
import { PageMapSum } from "./PageMapSum";

const stateIcon: Record<CellState["status"], IconName> = { starting: "spinner", filling: "spinner", filled: "check", stale: "alert", failed: "alert" };

// MapView is the page's map, beside it: its outline, its cells and where
// each stands, what's open, and the files it rests on. Each goes to what
// it names.
export function MapView({ map, passes, toPass, goTo, openCell }: { map: MapData; passes: PagePass[]; toPass: (name: string) => void; goTo: (pos: number) => void; openCell: (cell: string) => void }) {
  const ran = passes.filter((p) => p.runs || p.running);
  const stale = map.cells.filter((c) => c.state.status === "stale").length;
  const filling = map.cells.filter((c) => c.state.status === "filling" || c.state.status === "starting").length;
  const failed = map.cells.filter((c) => c.state.status === "failed").length;
  const nothing = !map.cells.length && !map.open.length && map.outline.length < 2 && !ran.length;
  return (
    <PageMap>
      {(map.open.length > 0 || stale > 0 || filling > 0 || failed > 0) && (
        <PageMapSum>
          {map.open.length > 0 && <Chip tone="attention">{map.open.length} open</Chip>}
          {stale > 0 && <Chip tone="attention">{stale} out of date</Chip>}
          {failed > 0 && <Chip tone="danger">{failed} not filled</Chip>}
          {filling > 0 && (
            <Chip tone="progress">
              <Spinner size={11} /> {filling} filling
            </Chip>
          )}
        </PageMapSum>
      )}
      {nothing && <PageMapEmpty />}
      {map.outline.length > 1 && (
        <PageMapSection title="Outline">
          {map.outline.map((h) => (
            <li key={h.pos} style={{ paddingLeft: (Math.min(h.level, 4) - 1) * 10 }}>
              <PageMapItem onClick={() => goTo(h.pos)}>{h.text || "Untitled"}</PageMapItem>
            </li>
          ))}
        </PageMapSection>
      )}
      {map.cells.length > 0 && (
        <PageMapSection title="Cells">
          {map.cells.map((c) => (
            <li key={c.name}>
              <PageMapItem
                look={c.state.status}
                title={c.state.note ? `${c.prompt}: ${c.state.note}` : c.prompt}
                onClick={() => {
                  goTo(c.pos);
                  if (c.state.status === "stale" || c.state.status === "failed") openCell(c.name);
                }}
              >
                {c.state.status === "filling" || c.state.status === "starting" ? <Spinner size={11} /> : <Icon name={stateIcon[c.state.status]} size={11} />}
                <span>{clip(c.prompt, 70)}</span>
              </PageMapItem>
            </li>
          ))}
        </PageMapSection>
      )}
      {ran.length > 0 && (
        <PageMapSection title="Passes">
          {ran.map((p) => {
            const open = p.findings.filter((f) => f.state === "open").length;
            return (
              <li key={p.name}>
                <PageMapItem look={p.running ? "filling" : p.moved ? "stale" : "filled"} title={p.about} onClick={() => toPass(p.name)}>
                  {p.running ? <Spinner size={11} /> : <Icon name={p.moved ? "alert" : "check"} size={11} />}
                  <span>
                    {p.title}
                    {p.running ? ": running" : `: ${open} open${p.moved ? ", the page has changed since" : ""}`}
                  </span>
                </PageMapItem>
              </li>
            );
          })}
        </PageMapSection>
      )}
      {map.open.length > 0 && (
        <PageMapSection title="Open">
          {map.open.map((o) => (
            <li key={o.pos}>
              <PageMapItem look="open" title={o.text} onClick={() => goTo(o.pos)}>
                {clip(o.text, 90)}
              </PageMapItem>
            </li>
          ))}
        </PageMapSection>
      )}
      {map.sources.length > 0 && (
        <PageMapSection title="Rests on">
          {map.sources.map(({ src, cells }) => {
            const name = sourceName(src);
            const title = `${name}${src.state === "gone" ? ": gone since" : src.state ? ": changed since" : ""}${cells > 1 ? ` (${cells} cells)` : ""}`;
            return (
              <li key={src.path}>
                <PageMapItem to={sourceHref(src)} look={src.state ? "moved" : undefined} title={title}>
                  <Icon name={src.state ? "alert" : "file"} size={11} />
                  <span>{name}</span>
                  {cells > 1 && <span className="dim">×{cells}</span>}
                </PageMapItem>
              </li>
            );
          })}
        </PageMapSection>
      )}
    </PageMap>
  );
}
