import { useNavigate } from "react-router";
import type { DocHit } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Rows } from "../ui/Rows";
import { DocumentsEmpty } from "./DocumentsEmpty";
import { DocumentsHitRow } from "./DocumentsHitRow";
import { hitTo } from "./useDocSearch";

// Results is what a search of every space found, in place of Documents'
// lists: a DocumentsHitRow each, the keys' cursor on one (index).
export function Results({ hits, more, error, index, setIndex }: { hits?: DocHit[]; more: number; error?: string; index: number; setIndex: (i: number) => void }) {
  const navigate = useNavigate();
  const menu = useThingMenu();
  if (error) return <ErrorCard title="Could not search" error={error} />;
  if (!hits) return <DocumentsEmpty>Searching…</DocumentsEmpty>;
  if (!hits.length) return <DocumentsEmpty>No matching documents.</DocumentsEmpty>;
  return (
    <section aria-label="Found">
      <GroupHead count={hits.length + more}>Found</GroupHead>
      <Rows role="listbox" aria-label="Documents found">
        {hits.map((h, i) => (
          <DocumentsHitRow
            key={`${h.repo}:${h.path}`}
            id={`hit-${i}`}
            to={hitTo(h)}
            selected={i === index}
            onHover={() => i !== index && setIndex(i)}
            onOpen={(e) => (e.preventDefault(), navigate(hitTo(h)))}
            onContextMenu={menu(hitTo(h))}
            title={h.title}
            heading={h.heading}
            state={h.state}
            every={!!h.every}
            snippet={h.snippet}
            where={h.label}
          />
        ))}
      </Rows>
      {more > 0 && <DocumentsEmpty>{more} more: refine your search.</DocumentsEmpty>}
    </section>
  );
}
